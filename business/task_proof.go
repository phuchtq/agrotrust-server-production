package business

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"raise-child/constants/env"
	"raise-child/constants/noti"
	"raise-child/constants/shared"
	"raise-child/interfaces/business"
	i_repository "raise-child/interfaces/repository"
	"raise-child/model/dtos/request"
	"raise-child/model/dtos/response"
	"raise-child/model/entities"
	"raise-child/repository"
	"raise-child/util"
	"raise-child/util/cache"
	"raise-child/util/db"
	on_chain "raise-child/util/on_chain"
	"strings"
	"time"

	"github.com/block-vision/sui-go-sdk/constant"
	"github.com/block-vision/sui-go-sdk/sui"
)

type taskProofService struct {
	profileRepo         i_repository.IProfileRepository
	taskProofRepo       i_repository.ITaskProofRepository
	childTaskDetailRepo i_repository.IChildTaskDetailRepository
	taskRepo            i_repository.ITaskRepository
	redisCache          cache.IRedisCache
	clients             map[string]sui.ISuiAPI
	errLogger           *log.Logger
}

func initializeTaskProofService(
	profileRepo i_repository.IProfileRepository,
	taskProofRepo i_repository.ITaskProofRepository,
	childTaskDetailRepo i_repository.IChildTaskDetailRepository,
	taskRepo i_repository.ITaskRepository,
	clients map[string]sui.ISuiAPI,
	errLogger *log.Logger,
) business.ITaskProofService {
	return &taskProofService{
		profileRepo:         profileRepo,
		taskProofRepo:       taskProofRepo,
		childTaskDetailRepo: childTaskDetailRepo,
		taskRepo:            taskRepo,
		redisCache:          cache.InitializeRedisCache(),
		clients:             clients,
		errLogger:           errLogger,
	}
}

func GenerateTaskProofService() (business.ITaskProofService, error) {
	var errLogger = util.GetLogConfig(shared.ERROR_LEVEL)

	cnn, err := db.ConnectDB(errLogger, db.InitializePostgreSQL())
	if err != nil {
		return nil, err
	}

	return initializeTaskProofService(
		repository.InitializeProfileRepository(cnn, errLogger),
		repository.InitializeTaskProofRepository(cnn, errLogger),
		repository.InitializeChildTaskDetailRepository(cnn, errLogger),
		repository.InitializeTaskRepository(cnn, errLogger),
		_networkAliases,
		errLogger,
	), nil
}

// ApproveTaskProof implements business.ITaskProofService.
func (t *taskProofService) ApproveTaskProof(id string, ctx context.Context) (response.BuildTransactionResponse, error) {
	proof, err := t.taskProofRepo.GetTaskProof(id, ctx)
	if err != nil {
		return response.BuildTransactionResponse{}, err
	}

	var genericErr error = errors.New(noti.GENERIC_ERROR_WARN_MSG)
	if proof == nil {
		return response.BuildTransactionResponse{}, genericErr
	}

	if proof.ReviewStatus != request_pending_status {
		return response.BuildTransactionResponse{}, errors.New(noti.TASK_PROOF_REVIEWED_MESSAGE)
	}

	task, err := t.taskRepo.GetTask(proof.TaskID, ctx)
	if err != nil {
		return response.BuildTransactionResponse{}, err
	}

	var sender string = ctx.Value("address").(string)
	var staffModule = on_chain.InitializeModuleStaff()
	var client = t.clients[constant.SuiTestnet]
	staffNfts, err := on_chain.GetOnChainOwnedObjects[entities.StaffNft](on_chain.GetOnChainOwnedObjectsRequest{
		Client:       client,
		OwnerAddress: sender,
		StructType:   fmt.Sprintf("%s::%s::%s", os.Getenv(env.PACKAGE_ID), staffModule.GetModule, staffModule.GetStaffNftObjectStruct()),
		ErrLogger:    t.errLogger,
	}, ctx)
	if err != nil {
		return response.BuildTransactionResponse{}, err
	}

	if staffNfts == nil || len(staffNfts) == 0 {
		return response.BuildTransactionResponse{}, errors.New(noti.GENERIC_RIGHT_ACCESS_WARN_MSG)
	}

	var leaderNftId string
	for _, nft := range staffNfts {
		if nft.Region == task.Region && nft.Role == local_leader_role {
			leaderNftId = nft.ID.ID
			break
		}
	}

	if leaderNftId == "" {
		return response.BuildTransactionResponse{}, errors.New(noti.GENERIC_RIGHT_ACCESS_WARN_MSG)
	}

	profile, err := t.profileRepo.GetProfile(ctx.Value("sub").(string), ctx)
	if err != nil {
		return response.BuildTransactionResponse{}, err
	}

	if profile.Status == "Suspended" {
		return response.BuildTransactionResponse{}, errors.New(noti.CURRENTLY_SUSPENDED_MESSAGE)
	}

	var args []interface{}
	var function string
	var childModule = on_chain.InitializeModuleChild()
	if task.IsChildTask {
		detail, err := t.childTaskDetailRepo.GetChildTaskDetail(*task.ChildTaskDetailID, ctx)
		if err != nil {
			return response.BuildTransactionResponse{}, err
		}

		switch detail.Purpose {
		case string(entities.MEAL_NEED_PURPOSE):
			function = childModule.GetFunctionConfirmProvideMealForChildV2()
			args = childModule.ToConfirmProvideMealForChildArgumentsV2(on_chain.ConfirmProvideMealForChildArgumentsV2{
				ChildID:     detail.ChildID,
				NeedID:      detail.Target,
				StaffNft:    leaderNftId,
				ImageBlobID: proof.ImageBlobID,
				ProvideDate: proof.RawSubmitDate,
				Actor:       proof.ActorAddress,
			})
			// Other cases in future if have
		}
	} else {
		manageObj, err := on_chain.GetOnChainObject[entities.Manage](on_chain.GetOnChainObjectRequest{
			Client:    client,
			ObjectId:  os.Getenv(env.MANAGE_OBJECT_ID),
			ErrLogger: t.errLogger,
		}, ctx)
		if err != nil {
			return response.BuildTransactionResponse{}, err
		}

		var centerId string
		for i, region := range manageObj.LocalRegions {
			if region == task.Region {
				centerId = manageObj.ChildrenCenters[i]
				break
			}
		}

		function = childModule.GetFunctionSubmitTask()
		args = childModule.ToSubmitTaskArguments(on_chain.SubmitTaskArguments{
			Center:      centerId,
			StaffNft:    leaderNftId,
			Description: task.Description,
			ImageBlobID: proof.ImageBlobID,
			Actor:       proof.ActorAddress,
		})
	}

	txBytes, err := on_chain.BuildTransaction(on_chain.BuildTransactionRequest{
		Client:    client,
		Sender:    sender,
		Module:    childModule.GetModule(),
		Function:  function,
		Arguments: args,
		ErrLogger: t.errLogger,
	}, ctx)
	if err != nil {
		return response.BuildTransactionResponse{}, err
	}

	proof.ReviewedBy = &sender
	proof.ReviewStatus = request_approved_status

	return response.BuildTransactionResponse{
		TxBytes: txBytes,
	}, t.taskProofRepo.UpdateTaskProof(*proof, ctx)
}

// GetTaskProof implements business.ITaskProofService.
func (t *taskProofService) GetTaskProof(id string, ctx context.Context) (*entities.TaskProof, error) {
	// return t.taskProofRepo.GetTaskProof(id, ctx)

	for _, proof := range mockTaskProofs {
		if proof.ID == id {
			return &proof, nil
		}
	}

	return nil, nil
}

// GetTaskProofs implements business.ITaskProofService.
func (t *taskProofService) GetTaskProofs(req request.GetTaskProofsRequest, ctx context.Context) (response.PaginationDataResponse, error) {
	// req.SortOrder = util.StanderizeSortOrder(req.SortOrder)
	// req.Keyword = strings.TrimSpace(req.Keyword)
	// if req.Page < 1 {
	// 	req.Page = 1
	// }

	// if req.PageSize < 1 {
	// 	req.PageSize = default_page_size
	// }

	// var genericErr error = errors.New(noti.GENERIC_ERROR_WARN_MSG)
	// if req.ActorAddress != "" {
	// 	if !util.IsValidSuiAddressStrict(req.ActorAddress) {
	// 		return response.PaginationDataResponse{}, genericErr
	// 	}
	// }

	// if req.ReviewedBy != "" {
	// 	if !util.IsValidSuiAddressStrict(req.ReviewedBy) {
	// 		return response.PaginationDataResponse{}, genericErr
	// 	}
	// }

	// var res response.PaginationDataResponse
	// var redisKey string = t.getGetTaskProofsRedisKey(req)
	// if t.redisCache.Get(redisKey, &res, ctx) {
	// 	return res, nil
	// }

	// data, pages, err := t.taskProofRepo.GetTaskProofs(req, ctx)
	// if err != nil {
	// 	return response.PaginationDataResponse{}, err
	// }

	// var amount int
	// if data == nil || len(data) == 0 {
	// 	amount = 0
	// } else {
	// 	amount = len(data)
	// }

	// res = response.PaginationDataResponse{
	// 	Data:       data,
	// 	Amount:     amount,
	// 	Page:       req.Page,
	// 	TotalPages: pages,
	// }

	// t.redisCache.Set(redisKey, res, time.Minute*5, ctx)

	// return res, nil

	////////////////////////
	// MOCK DATA
	req.SortOrder = util.StanderizeSortOrder(req.SortOrder)
	req.Keyword = strings.TrimSpace(req.Keyword)
	if req.Page < 1 {
		req.Page = 1
	}

	if req.PageSize < 1 {
		req.PageSize = default_page_size
	}

	var res response.PaginationDataResponse
	var redisKey string = t.getGetTaskProofsRedisKey(req)
	if t.redisCache.Get(redisKey, &res, ctx) {
		return res, nil
	}

	var data []entities.TaskProof = mockTaskProofs[req.Page-1*req.PageSize : req.Page*req.PageSize]
	res = response.PaginationDataResponse{
		Data:       data,
		Amount:     len(data),
		Page:       req.Page,
		TotalPages: int(math.Ceil(float64(len(data)) / float64(req.PageSize))),
	}

	t.redisCache.Set(redisKey, res, time.Minute*5, ctx)

	return res, nil
}

// RefuseTaskProof implements business.ITaskProofService.
func (t *taskProofService) RefuseTaskProof(id string, ctx context.Context) error {
	proof, err := t.taskProofRepo.GetTaskProof(id, ctx)
	if err != nil {
		return err
	}

	var genericErr error = errors.New(noti.GENERIC_ERROR_WARN_MSG)
	if proof == nil {
		return genericErr
	}

	if proof.ReviewStatus != request_pending_status {
		return errors.New(noti.TASK_PROOF_REVIEWED_MESSAGE)
	}

	task, err := t.taskRepo.GetTask(proof.TaskID, ctx)
	if err != nil {
		return err
	}

	var sender string = ctx.Value("address").(string)
	var staffModule = on_chain.InitializeModuleStaff()
	staffNfts, err := on_chain.GetOnChainOwnedObjects[entities.StaffNft](on_chain.GetOnChainOwnedObjectsRequest{
		Client:       t.clients[constant.SuiTestnet],
		OwnerAddress: sender,
		StructType:   fmt.Sprintf("%s::%s::%s", os.Getenv(env.PACKAGE_ID), staffModule.GetModule, staffModule.GetStaffNftObjectStruct()),
		ErrLogger:    t.errLogger,
	}, ctx)
	if err != nil {
		return err
	}

	if staffNfts == nil || len(staffNfts) == 0 {
		return errors.New(noti.GENERIC_RIGHT_ACCESS_WARN_MSG)
	}

	var isLeaderOfRegion bool = false
	for _, nft := range staffNfts {
		if nft.Region == task.Region && nft.Role == local_leader_role {
			isLeaderOfRegion = true
			break
		}
	}

	if !isLeaderOfRegion {
		return errors.New(noti.GENERIC_RIGHT_ACCESS_WARN_MSG)
	}

	profile, err := t.profileRepo.GetProfile(ctx.Value("sub").(string), ctx)
	if err != nil {
		return err
	}

	if profile.Status == "Suspended" {
		return errors.New(noti.CURRENTLY_SUSPENDED_MESSAGE)
	}

	proof.ReviewedBy = &sender
	proof.ReviewStatus = request_refused_status

	return t.taskProofRepo.UpdateTaskProof(*proof, ctx)
}

// SubmitTaskProof implements business.ITaskProofService.
func (t *taskProofService) SubmitTaskProof(id string, req request.SubmitTaskProofRequest, ctx context.Context) error {
	task, err := t.taskRepo.GetTask(id, ctx)
	if err != nil {
		return err
	}

	var genericErr error = errors.New(noti.GENERIC_ERROR_WARN_MSG)
	if task == nil {
		return genericErr
	}

	if time.Now().After(task.EndPeriod) {
		return errors.New(noti.TASK_ENDED_MESSAGE)
	}

	var sender string = ctx.Value("address").(string)
	if task.AssignedStaff == nil {
		return errors.New(noti.TASK_NOT_CLAIMED_MESSAGE)
	} else {
		if task.AssignedProfileID != &sender {
			return errors.New(noti.TASK_NOT_OF_STAFF_MESSAGE)
		}
	}

	var curTime time.Time = time.Now()
	var rawSubmitDate string = util.TimeToRawDate(curTime)
	isSubmitted, err := t.taskProofRepo.IsTaskProofSumittedWithDetail(id, task.Description, sender, rawSubmitDate, ctx)
	if err != nil {
		return err
	}

	if isSubmitted {
		return errors.New(noti.TASK_PROOF_SUBMITTED_MESSAGE)
	}

	// todo: AI validation
	return t.taskProofRepo.CreateTaskProof(entities.TaskProof{
		ID:             util.GenerateId(),
		TaskID:         id,
		Description:    task.Description,
		ActorProfileID: ctx.Value("sub").(string),
		ActorAddress:   sender,
		ImageBlobID:    req.ImageBlobID,
		AIEvaluation:   "",
		RawSubmitDate:  rawSubmitDate,
		CreatedAt:      curTime,
		UpdatedAt:      curTime,
	}, ctx)
}

func (t *taskProofService) getGetTaskProofsRedisKey(req request.GetTaskProofsRequest) string {
	var keyword string = "empty"
	if req.Keyword != "" {
		keyword = req.Keyword
	}

	var status string = "empty"
	if req.Status != "" {
		status = req.Status
	}

	var actorAddress string = "empty"
	if req.ActorAddress != "" {
		actorAddress = req.ActorAddress
	}

	var reviewedBy string = "empty"
	if req.ReviewedBy != "" {
		reviewedBy = req.ReviewedBy
	}

	return fmt.Sprintf("task:kw:%s:status:%s:actor:%s:reviewed:%s:o:%s:s:%d:p:%d",
		keyword, status, actorAddress, reviewedBy, req.SortOrder, req.PageSize, req.Page)
}

var mockTaskProofs = getMockTaskProofs()

func getMockTaskProofs() []entities.TaskProof {
	proofs := make([]entities.TaskProof, 20)
	now := time.Now()

	// Dữ liệu mẫu để xoay vòng
	evaluations := []string{"High Confidence", "Low Confidence", "Manual Review Required", "Verified by AI"}
	admins := []string{"Admin_01", "Admin_02", "Moderator_X"}
	var ptrStr = func(s string) *string {
		return &s
	}

	for i := 0; i < 20; i++ {
		id := fmt.Sprintf("%d", i+1)

		var status string
		var reviewedBy *string

		// Logic phân bổ trạng thái: 0=Pending, 1=Approved, 2=Refused
		switch i % 3 {
		case 0:
			status = "Pending"
			reviewedBy = nil
		case 1:
			status = "Approved"
			reviewedBy = ptrStr(admins[i%3])
		case 2:
			status = "Refused"
			reviewedBy = ptrStr(admins[i%3])
		}

		proofs[i] = entities.TaskProof{
			ID:             id,
			TaskID:         fmt.Sprintf("TASK-%03d", (i/2)+1),
			Description:    fmt.Sprintf("Báo cáo hoàn thành công việc đợt %d", i+1),
			ActorProfileID: fmt.Sprintf("ACT-%03d", i+100),
			ActorAddress:   fmt.Sprintf("0x%x...%x", 123456+i, 789+i),
			ImageBlobID:    fmt.Sprintf("img-proof-blob-%d", i+1),
			ReviewedBy:     reviewedBy,
			AIEvaluation:   evaluations[i%4],
			ReviewStatus:   status,
			RawSubmitDate:  now.Format("2006-01-02"),
			CreatedAt:      now.Add(time.Duration(-i) * time.Hour),
			UpdatedAt:      now,
		}
	}
	return proofs
}
