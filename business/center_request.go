package business

import (
	"context"
	"errors"
	"fmt"
	"log"
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
	"slices"
	"strings"
	"time"

	"github.com/block-vision/sui-go-sdk/constant"
	"github.com/block-vision/sui-go-sdk/models"
	"github.com/block-vision/sui-go-sdk/sui"
	"github.com/block-vision/sui-go-sdk/utils"
)

type centerRequestService struct {
	centerRequestRepo i_repository.ICenterRequestRepository
	profileRepo       i_repository.IProfileRepository
	redisCache        cache.IRedisCache
	clients           map[string]sui.ISuiAPI
	errLogger         *log.Logger
}

const (
	default_page_size int = 10
)

var (
	min_region_staffs int = 5
)

func initializeCenterRequestService(
	centerRequestRepo i_repository.ICenterRequestRepository,
	profileRepo i_repository.IProfileRepository,
	clients map[string]sui.ISuiAPI,
	errLogger *log.Logger,
) business.ICenterRequestService {
	return &centerRequestService{
		centerRequestRepo: centerRequestRepo,
		profileRepo:       profileRepo,
		redisCache:        cache.InitializeRedisCache(),
		clients:           clients,
		errLogger:         errLogger,
	}
}

func GenerateCenterRequestService() (business.ICenterRequestService, error) {
	var errLogger = util.GetLogConfig(shared.ERROR_LEVEL)

	cnn, err := db.ConnectDB(errLogger, db.InitializePostgreSQL())
	if err != nil {
		return nil, err
	}

	return initializeCenterRequestService(
		repository.InitializeCenterRequestRepository(cnn, errLogger),
		repository.InitializeProfileRepository(cnn, errLogger),
		_networkAliases,
		errLogger,
	), nil
}

// ConfirmRequest implements business.ICenterRequestService.
func (c *centerRequestService) ConfirmRequest(id string, ctx context.Context) (response.BuildTransactionResponse, error) {
	var genericErr error = errors.New(noti.GENERIC_ERROR_WARN_MSG)

	var sender string = ctx.Value("address").(string)
	if !utils.IsValidSuiAddress(models.SuiAddress(sender)) {
		return response.BuildTransactionResponse{}, genericErr
	}

	req, err := c.centerRequestRepo.GetRequest(id, ctx)
	if err != nil {
		return response.BuildTransactionResponse{}, err
	}

	if req == nil {
		return response.BuildTransactionResponse{}, genericErr
	}

	if req.CreatedBy != sender {
		return response.BuildTransactionResponse{}, errors.New(noti.GENERIC_RIGHT_ACCESS_WARN_MSG)
	}

	// Pending process
	if req.ClosedAt.After(time.Now()) {
		return response.BuildTransactionResponse{}, errors.New(noti.STILL_PENDING_REQUEST_MESSAGE)
	}

	var rate float32 = float32(len(req.Approvers)) / float32(len(req.Approvers)+len(req.Refusers))
	// var isDenied bool = false

	// if rate >= approve_rate_limit {
	// 	req.Status = request_approved_status
	// 	req.IsConfirmRegister = true
	// 	if req.IsAvailableToConfirm {
	// 		req.IsConfirmRegister = true
	// 	}
	// } else {
	// 	req.Status = request_refused_status
	// 	isDenied = true
	// }

	// req.UpdatedAt = time.Now()
	// if err := c.centerRequestRepo.UpdateRegistrationRequest(*req, ctx); err != nil {
	// 	return response.BuildTransactionResponse{}, err
	// }

	// if isDenied {
	// 	return response.BuildTransactionResponse{}, nil
	// }

	if rate < approve_rate_limit {
		req.Status = request_refused_status
		req.UpdatedAt = time.Now()
		if err := c.centerRequestRepo.UpdateRegistrationRequest(*req, ctx); err != nil {
			return response.BuildTransactionResponse{}, err
		}

		return response.BuildTransactionResponse{}, nil
	}

	// Wait for background server to mint cap object to register
	if !req.IsAvailableToConfirm {
		return response.BuildTransactionResponse{}, nil
	}

	var manageModule = on_chain.InitializeModuleManage()
	var client = c.clients[constant.SuiTestnet]
	caps, err := on_chain.GetOnChainOwnedObjects[entities.Cap](on_chain.GetOnChainOwnedObjectsRequest{
		Client:       client,
		OwnerAddress: sender,
		StructType:   fmt.Sprintf("%s::%s::%s", os.Getenv(env.PACKAGE_ID), manageModule.GetModule(), manageModule.GetUploadCenterCapStruct()),
	}, ctx)
	if err != nil {
		return response.BuildTransactionResponse{}, err
	}

	if caps == nil || len(caps) == 0 {
		return response.BuildTransactionResponse{}, genericErr
	}

	var manageObj entities.Manage
	if !c.redisCache.Get(manageObj.GetRedisKey(), &manageObj, ctx) {
		res, err := on_chain.GetOnChainObject[entities.Manage](on_chain.GetOnChainObjectRequest{
			Client:    client,
			ObjectId:  os.Getenv(env.MANAGE_OBJECT_ID),
			ErrLogger: c.errLogger,
		}, ctx)
		if err != nil {
			return response.BuildTransactionResponse{}, err
		}

		if res != nil {
			c.redisCache.Set(manageObj.GetRedisKey(), res, time.Minute, ctx)
			manageObj = *res
		}
	}

	var startIdx int
	for i, region := range manageObj.LocalRegions {
		if region == req.Region {
			startIdx = i
			break
		}
	}

	leaders, err := on_chain.GetOnChainObjects[entities.StaffNft](on_chain.GetOnChainObjectsRequest{
		Client:    client,
		ObjectIds: manageObj.LocalLeaderNfts[startIdx:],
		ErrLogger: c.errLogger,
	}, ctx)
	if err != nil {
		return response.BuildTransactionResponse{}, err
	}

	var leaderAddresses []string
	for i, leader := range leaders {
		if leader.Region == req.Region {
			leaderAddresses = append(leaderAddresses, manageObj.LocalLeaderIds[startIdx+i])
		}
	}

	var childModule = on_chain.InitializeModuleChild()
	txBytes, err := on_chain.BuildTransaction(on_chain.BuildTransactionRequest{
		Client:    client,
		Sender:    sender,
		Module:    childModule.GetModule(),
		Function:  childModule.GetFunctionUploadCenter(),
		ErrLogger: c.errLogger,
		Arguments: childModule.ToCreateCenterArguments(on_chain.CreateCenterArguments{
			CapID:       caps[0].ID.ID,
			Region:      req.Region,
			Address:     req.Address,
			PhoneNumber: req.PhoneNumber,
			ImageBlobID: req.ImageBlobID,
			Leaders:     leaderAddresses,
		}),
	}, ctx)

	return response.BuildTransactionResponse{
		TxBytes: txBytes,
	}, err
}

// CreateRequest implements business.ICenterRequestService.
func (c *centerRequestService) CreateRequest(req request.CreateCenterRequest, ctx context.Context) (*entities.CenterRequest, error) {
	var genericErr error = errors.New(noti.GENERIC_ERROR_WARN_MSG)

	var sender string = ctx.Value("address").(string)
	if !utils.IsValidSuiAddress(models.SuiAddress(sender)) {
		return nil, genericErr
	}

	var client = c.clients[constant.SuiTestnet]
	var module = on_chain.InitializeModuleStaff()
	staffNfts, err := on_chain.GetOnChainOwnedObjects[entities.StaffNft](on_chain.GetOnChainOwnedObjectsRequest{
		Client:       client,
		OwnerAddress: sender,
		StructType:   fmt.Sprintf("%s::%s::%s", os.Getenv(env.PACKAGE_ID), module.GetModule(), module.GetStaffNftObjectStruct()),
		ErrLogger:    c.errLogger,
	}, ctx)
	if err != nil {
		return nil, err
	}

	var genericRightErr error = errors.New(noti.GENERIC_RIGHT_ACCESS_WARN_MSG)
	if staffNfts == nil || len(staffNfts) == 0 {
		return nil, genericRightErr
	}

	var isRegionStaff bool = false
	for _, nft := range staffNfts {
		if nft.Region == req.Region {
			isRegionStaff = true
			break
		}
	}

	if !isRegionStaff {
		return nil, genericRightErr
	}

	var manageObj entities.Manage
	if !c.redisCache.Get(manageObj.GetRedisKey(), &manageObj, ctx) {
		res, err := on_chain.GetOnChainObject[entities.Manage](on_chain.GetOnChainObjectRequest{
			Client:    client,
			ObjectId:  os.Getenv(env.MANAGE_OBJECT_ID),
			ErrLogger: c.errLogger,
		}, ctx)
		if err != nil {
			return nil, err
		}

		if res != nil {
			c.redisCache.Set(manageObj.GetRedisKey(), res, time.Minute, ctx)
			manageObj = *res
		}
	}

	var isRegionHaveStaffs bool = false
	for i := 0; i < len(manageObj.LocalRegions); i++ {
		if manageObj.LocalRegions[i] == req.Region && !manageObj.CenterConfirmStatuses[i] {
			isRegionHaveStaffs = true
			break
		}
	}

	if !isRegionHaveStaffs {
		return nil, genericErr
	}

	var nftIds []string = append(manageObj.LocalLeaderNfts, manageObj.VolunteerNfts...)
	nfts, err := on_chain.GetOnChainObjects[entities.StaffNft](on_chain.GetOnChainObjectsRequest{
		Client:    client,
		ObjectIds: nftIds,
		ErrLogger: c.errLogger,
	}, ctx)
	if err != nil {
		return nil, err
	}

	var staffCount int = 0
	for _, nft := range nfts {
		if nft.Region == req.Region {
			staffCount++
			if staffCount == min_region_staffs {
				break
			}
		}
	}

	if staffCount < min_region_staffs {
		return nil, genericErr
	}

	// todo: AI validate address if matched with region
	var address string = strings.TrimSpace(req.Address)
	var phoneNumber string = strings.TrimSpace(req.PhoneNumber)
	var curTime time.Time = time.Now()
	var request = entities.CenterRequest{
		ID:          util.GenerateId(),
		ProfileID:   ctx.Value("sub").(string),
		Region:      req.Region,
		Address:     address,
		PhoneNumber: phoneNumber,
		ImageBlobID: req.ImageBlobID,
		Status:      request_pending_status,
		CreatedBy:   sender,
		CreatedAt:   curTime,
		UpdatedAt:   curTime,
		ClosedAt:    util.GetRequestDuration(),
	}

	return &request, c.centerRequestRepo.CreateRegistrationRequest(request, ctx)
}

// GetRequest implements business.ICenterRequestService.
func (c *centerRequestService) GetRequest(id string, ctx context.Context) (*entities.CenterRequest, error) {
	var data entities.CenterRequest
	var redisKey string = c.getCenterRequestRedisKey(id)
	if c.redisCache.Get(redisKey, &data, ctx) {
		return &data, nil
	}

	res, err := c.centerRequestRepo.GetRequest(id, ctx)
	if err != nil {
		return nil, err
	}

	return res, nil

	// /////////////////////
	// // MOCK DATA
	// var data entities.CenterRequest
	// var redisKey string = c.getCenterRequestRedisKey(id)
	// if c.redisCache.Get(redisKey, &data, ctx) {
	// 	return &data, nil
	// }

	// for _, req := range mockCenterRequests {
	// 	if req.ID == id {
	// 		c.redisCache.Set(redisKey, req, time.Minute, ctx)
	// 		return &req, nil
	// 	}
	// }

	// return nil, nil
}

// GetRequests implements business.ICenterRequestService.
func (c *centerRequestService) GetRequests(req request.GetCenterRequests, ctx context.Context) (response.PaginationDataResponse, error) {
	if req.Page < 1 {
		req.Page = 1
	}

	if req.PageSize < 1 {
		req.PageSize = default_page_size
	}

	var res response.PaginationDataResponse
	// var redisKey string = c.getGetCenterRequestsRedisKey(req)
	// if c.redisCache.Get(redisKey, &res, ctx) {
	// 	return res, nil
	// }

	data, pages, err := c.centerRequestRepo.GetRegistrationRequests(req, ctx)
	var amount int
	if data == nil || len(data) == 0 {
		amount = 0
	} else {
		amount = len(data)
	}

	res = response.PaginationDataResponse{
		Data:       data,
		Amount:     amount,
		Page:       req.Page,
		TotalPages: pages,
	}

	//c.redisCache.Set(redisKey, res, time.Minute*5, ctx)

	return res, err

	// //////////////////
	// // MOCK DATA
	// if req.Page < 1 {
	// 	req.Page = 1
	// }

	// if req.PageSize < 1 {
	// 	req.PageSize = default_page_size
	// }

	// var res response.PaginationDataResponse
	// var redisKey string = c.getGetCenterRequestsRedisKey(req)
	// if c.redisCache.Get(redisKey, &res, ctx) {
	// 	return res, nil
	// }

	// var data []entities.CenterRequest = mockCenterRequests[(req.Page-1)*req.PageSize : req.Page*req.PageSize]
	// res = response.PaginationDataResponse{
	// 	Data:       data,
	// 	Amount:     len(data),
	// 	Page:       req.Page,
	// 	TotalPages: int(math.Ceil(float64(len(mockCenterRequests)) / float64(req.PageSize))),
	// }

	// c.redisCache.Set(redisKey, res, time.Minute*5, ctx)

	// return res, nil
}

// GetWalletRequests implements business.ICenterRequestService.
func (c *centerRequestService) GetWalletRequests(id string, ctx context.Context) ([]entities.CenterRequest, error) {
	if !util.IsValidSuiAddressStrict(id) {
		return nil, errors.New(noti.GENERIC_ERROR_WARN_MSG)
	}

	var res []entities.CenterRequest
	// var redisKey string = c.getGetWalletCenterRequestsRedisKey(id)
	// if c.redisCache.Get(redisKey, &res, ctx) {
	// 	return res, nil
	// }

	var errRes error
	res, errRes = c.centerRequestRepo.GetWalletRegistrationRequests(id, ctx)
	if errRes != nil {
		return nil, errRes
	}

	//c.redisCache.Set(redisKey, res, time.Minute*5, ctx)

	return res, errRes

	//////////////////
	// // MOCK DATA
	// var res []entities.CenterRequest
	// var redisKey string = c.getGetWalletCenterRequestsRedisKey(id)
	// if c.redisCache.Get(redisKey, &res, ctx) {
	// 	return res, nil
	// }

	// for _, req := range mockCenterRequests {
	// 	if id == req.CreatedBy {
	// 		res = append(res, req)
	// 	}
	// }

	// c.redisCache.Set(redisKey, res, time.Minute*5, ctx)

	// return res, nil
}

// VoteRequest implements business.ICenterRequestService.
func (c *centerRequestService) VoteRequest(id string, req request.VoteRequest, ctx context.Context) error {
	var genericErr error = errors.New(noti.GENERIC_ERROR_WARN_MSG)

	var voter string = ctx.Value("address").(string)
	if !utils.IsValidSuiAddress(models.SuiAddress(voter)) {
		return genericErr
	}

	request, err := c.centerRequestRepo.GetRequest(id, ctx)
	if err != nil {
		return err
	}

	if request == nil {
		return genericErr
	}

	if request.ClosedAt.Before(time.Now()) {
		return errors.New(noti.REQUEST_CLOSED_MESSAGE)
	}

	if voter == request.CreatedBy {
		return errors.New(noti.OWNER_VOTE_WARN_MSG)
	}

	if slices.Contains(request.Approvers, voter) || slices.Contains(request.Refusers, voter) {
		return errors.New(noti.ALREADY_VOTE_MESSAGE)
	}

	var module = on_chain.InitializeModuleStaff()
	nfts, err := on_chain.GetOnChainOwnedObjects[entities.StaffNft](on_chain.GetOnChainOwnedObjectsRequest{
		Client:       c.clients[constant.SuiTestnet],
		OwnerAddress: voter,
		StructType:   fmt.Sprintf("%s::%s::%s", os.Getenv(env.PACKAGE_ID), module.GetModule(), module.GetStaffNftObjectStruct()),
		ErrLogger:    c.errLogger,
	}, ctx)
	if err != nil {
		return err
	}

	var genericRightErr error = errors.New(noti.GENERIC_RIGHT_ACCESS_WARN_MSG)
	if nfts == nil || len(nfts) == 0 {
		return genericRightErr
	}

	var isRegionStaff bool = false
	for _, nft := range nfts {
		if nft.Region == request.Region {
			isRegionStaff = true
			break
		}
	}

	if !isRegionStaff {
		return genericRightErr
	}

	if req.IsVoteYes {
		request.Approvers = append(request.Approvers, voter)
	} else {
		request.Refusers = append(request.Refusers, voter)
		if req.RefuseReason == "" {
			return errors.New(noti.FIELD_EMPTY_WARN_MSG)
		}

		request.RefuseReasons = append(request.RefuseReasons, strings.TrimSpace(req.RefuseReason))
	}

	request.UpdatedAt = time.Now()

	return c.centerRequestRepo.UpdateRegistrationRequest(*request, ctx)
}

// EditStaffNumbersToRequestCenter implements business.ICenterRequestService.
func (c *centerRequestService) EditStaffNumbersToRequestCenter(req request.EditStaffNumbersToCenterRequest, ctx context.Context) error {
	var module = on_chain.InitializeModuleManage()
	caps, err := on_chain.GetOnChainOwnedObjects[entities.Cap](on_chain.GetOnChainOwnedObjectsRequest{
		Client:       c.clients[constant.SuiTestnet],
		OwnerAddress: ctx.Value("address").(string),
		StructType:   fmt.Sprintf("%s::%s::%s", os.Getenv(env.PACKAGE_ID), module.GetModule(), module.GetAdminCapStruct()),
		ErrLogger:    c.errLogger,
	}, ctx)
	if err != nil {
		return err
	}

	if caps == nil || len(caps) == 0 {
		return errors.New(noti.GENERIC_RIGHT_ACCESS_WARN_MSG)
	}

	if req.MinStaffNumber != nil {
		if *req.MinStaffNumber > 0 {
			min_region_staffs = *req.MinStaffNumber
		} else {

		}
	}

	return nil
}

func (c *centerRequestService) getGetCenterRequestsRedisKey(req request.GetCenterRequests) string {
	var keyword string = "empty"
	if req.Keyword != "" {
		keyword = req.Keyword
	}

	var region string = "empty"
	if req.Region != "" {
		region = req.Region
	}

	var status string = "empty"
	if req.Status != "" {
		status = req.Status
	}

	var isClosed string = "empty"
	if req.IsClosed != nil {
		isClosed = fmt.Sprintf("%v", *req.IsClosed)
	}

	return fmt.Sprintf("center_rq:kw:%s:r:%s:status:%s:close:%s:o:%s:s:%d:p:%d",
		keyword, region, status, isClosed, req.SortOrder, req.PageSize, req.Page)
}

func (c *centerRequestService) getGetWalletCenterRequestsRedisKey(wallet string) string {
	return fmt.Sprintf("center_rq:wallet:%s", wallet)
}

func (c *centerRequestService) getCenterRequestRedisKey(id string) string {
	return fmt.Sprintf("center_rq:%s", id)
}

var global_cur_time = time.Now()
var mockCenterRequests = []entities.CenterRequest{
	{
		ID: "1", ProfileID: "PROF-001", Region: "Hà Nội", Address: "123 Đường Láng, Đống Đa",
		PhoneNumber: "0901112223", ImageBlobID: "img-001", Status: "Approved",
		Approvers: []string{"admin-01", "admin-02"}, IsAvailableToConfirm: true, IsConfirmRegister: true,
		CreatedBy: "user-01", CreatedAt: global_cur_time.Add(-100 * time.Hour), UpdatedAt: global_cur_time.Add(-24 * time.Hour),
	},
	{
		ID: "2", ProfileID: "PROF-002", Region: "TP.HCM", Address: "456 Lê Lợi, Quận 1",
		PhoneNumber: "0902223334", ImageBlobID: "img-002", Status: "Pending",
		Approvers: []string{}, IsAvailableToConfirm: false, IsConfirmRegister: false,
		CreatedBy: "user-02", CreatedAt: global_cur_time.Add(-90 * time.Hour), UpdatedAt: global_cur_time.Add(-80 * time.Hour),
	},
	{
		ID: "3", ProfileID: "PROF-003", Region: "Đà Nẵng", Address: "789 Võ Nguyên Giáp",
		PhoneNumber: "0903334445", ImageBlobID: "img-003", Status: "Refused",
		Refusers: []string{"admin-03"}, RefuseReasons: []string{"Ảnh chứng chỉ mờ"},
		IsAvailableToConfirm: false, IsConfirmRegister: false,
		CreatedBy: "user-03", CreatedAt: global_cur_time.Add(-80 * time.Hour), UpdatedAt: global_cur_time.Add(-70 * time.Hour),
	},
	{
		ID: "4", ProfileID: "PROF-004", Region: "Cần Thơ", Address: "12 Đại lộ Hòa Bình",
		PhoneNumber: "0904445556", ImageBlobID: "img-004", Status: "Approved",
		Approvers: []string{"admin-01"}, IsAvailableToConfirm: true, IsConfirmRegister: false,
		CreatedBy: "user-04", CreatedAt: global_cur_time.Add(-70 * time.Hour), UpdatedAt: global_cur_time.Add(-60 * time.Hour),
	},
	{
		ID: "5", ProfileID: "PROF-005", Region: "Hải Phòng", Address: "34 Lạch Tray",
		PhoneNumber: "0905556667", ImageBlobID: "img-005", Status: "Pending",
		IsAvailableToConfirm: false, IsConfirmRegister: false,
		CreatedBy: "user-05", CreatedAt: global_cur_time.Add(-60 * time.Hour), UpdatedAt: global_cur_time.Add(-50 * time.Hour),
	},
	{
		ID: "6", ProfileID: "PROF-006", Region: "Huế", Address: "56 Hùng Vương",
		PhoneNumber: "0906667778", ImageBlobID: "img-006", Status: "Refused",
		Refusers: []string{"admin-02"}, RefuseReasons: []string{"Địa chỉ không tồn tại"},
		CreatedBy: "user-06", CreatedAt: global_cur_time.Add(-50 * time.Hour), UpdatedAt: global_cur_time.Add(-40 * time.Hour),
	},
	{
		ID: "7", ProfileID: "PROF-007", Region: "Nha Trang", Address: "78 Trần Phú",
		PhoneNumber: "0907778889", ImageBlobID: "img-007", Status: "Approved",
		Approvers: []string{"admin-04"}, IsAvailableToConfirm: true, IsConfirmRegister: true,
		CreatedBy: "user-07", CreatedAt: global_cur_time.Add(-40 * time.Hour), UpdatedAt: global_cur_time.Add(-30 * time.Hour),
	},
	{
		ID: "8", ProfileID: "PROF-008", Region: "Đà Lạt", Address: "90 Phan Đình Phùng",
		PhoneNumber: "0908889990", ImageBlobID: "img-008", Status: "Pending",
		CreatedBy: "user-08", CreatedAt: global_cur_time.Add(-30 * time.Hour), UpdatedAt: global_cur_time.Add(-20 * time.Hour),
	},
	{
		ID: "9", ProfileID: "PROF-009", Region: "Vũng Tàu", Address: "23 Thùy Vân",
		PhoneNumber: "0909990001", ImageBlobID: "img-009", Status: "Approved",
		Approvers: []string{"admin-01", "admin-05"}, IsAvailableToConfirm: true, IsConfirmRegister: false,
		CreatedBy: "user-09", CreatedAt: global_cur_time.Add(-20 * time.Hour), UpdatedAt: global_cur_time.Add(-10 * time.Hour),
	},
	{
		ID: "10", ProfileID: "PROF-010", Region: "Hà Nội", Address: "11 Cầu Giấy",
		PhoneNumber: "0911223344", ImageBlobID: "img-010", Status: "Pending",
		CreatedBy: "user-10", CreatedAt: global_cur_time.Add(-10 * time.Hour), UpdatedAt: global_cur_time.Add(-5 * time.Hour),
	},
	{
		ID: "11", ProfileID: "PROF-011", Region: "TP.HCM", Address: "88 Nguyễn Huệ",
		PhoneNumber: "0912334455", ImageBlobID: "img-011", Status: "Refused",
		RefuseReasons: []string{"Số điện thoại không liên lạc được"},
		CreatedBy:     "user-11", CreatedAt: global_cur_time.Add(-15 * time.Hour), UpdatedAt: global_cur_time.Add(-2 * time.Hour),
	},
	{
		ID: "12", ProfileID: "PROF-012", Region: "Bắc Ninh", Address: "45 Lý Thái Tổ",
		PhoneNumber: "0913445566", ImageBlobID: "img-012", Status: "Approved",
		Approvers: []string{"admin-02"}, IsAvailableToConfirm: true, IsConfirmRegister: true,
		CreatedBy: "user-12", CreatedAt: global_cur_time.Add(-12 * time.Hour), UpdatedAt: global_cur_time.Add(-1 * time.Hour),
	},
	{
		ID: "13", ProfileID: "PROF-013", Region: "Quảng Ninh", Address: "67 Hạ Long",
		PhoneNumber: "0914556677", ImageBlobID: "img-013", Status: "Pending",
		CreatedBy: "user-13", CreatedAt: global_cur_time.Add(-8 * time.Hour), UpdatedAt: global_cur_time.Add(-4 * time.Hour),
	},
	{
		ID: "14", ProfileID: "PROF-014", Region: "Nghệ An", Address: "12 Vinh",
		PhoneNumber: "0915667788", ImageBlobID: "img-014", Status: "Pending",
		CreatedBy: "user-14", CreatedAt: global_cur_time.Add(-6 * time.Hour), UpdatedAt: global_cur_time.Add(-3 * time.Hour),
	},
	{
		ID: "15", ProfileID: "PROF-015", Region: "Thanh Hóa", Address: "34 Sầm Sơn",
		PhoneNumber: "0916778899", ImageBlobID: "img-015", Status: "Refused",
		RefuseReasons: []string{"Hồ sơ thiếu công chứng"},
		CreatedBy:     "user-15", CreatedAt: global_cur_time.Add(-24 * time.Hour), UpdatedAt: global_cur_time.Add(-12 * time.Hour),
	},
	{
		ID: "16", ProfileID: "PROF-016", Region: "Hà Giang", Address: "56 Đồng Văn",
		PhoneNumber: "0917889900", ImageBlobID: "img-016", Status: "Approved",
		Approvers: []string{"admin-03"}, IsAvailableToConfirm: true, IsConfirmRegister: false,
		CreatedBy: "user-16", CreatedAt: global_cur_time.Add(-48 * time.Hour), UpdatedAt: global_cur_time.Add(-24 * time.Hour),
	},
	{
		ID: "17", ProfileID: "PROF-017", Region: "Lào Cai", Address: "78 Sa Pa",
		PhoneNumber: "0918990011", ImageBlobID: "img-017", Status: "Pending",
		CreatedBy: "user-17", CreatedAt: global_cur_time.Add(-72 * time.Hour), UpdatedAt: global_cur_time.Add(-36 * time.Hour),
	},
	{
		ID: "18", ProfileID: "PROF-018", Region: "Phú Quốc", Address: "90 Dương Đông",
		PhoneNumber: "0919001122", ImageBlobID: "img-018", Status: "Approved",
		Approvers: []string{"admin-01"}, IsAvailableToConfirm: true, IsConfirmRegister: true,
		CreatedBy: "user-18", CreatedAt: global_cur_time.Add(-96 * time.Hour), UpdatedAt: global_cur_time.Add(-48 * time.Hour),
	},
	{
		ID: "19", ProfileID: "PROF-019", Region: "Bình Dương", Address: "12 Thủ Dầu Một",
		PhoneNumber: "0920112233", ImageBlobID: "img-019", Status: "Pending",
		CreatedBy: "user-19", CreatedAt: global_cur_time.Add(-120 * time.Hour), UpdatedAt: global_cur_time.Add(-60 * time.Hour),
	},
	{
		ID: "20", ProfileID: "PROF-020", Region: "Đồng Nai", Address: "34 Biên Hòa",
		PhoneNumber: "0921223344", ImageBlobID: "img-020", Status: "Refused",
		RefuseReasons: []string{"Thông tin đăng ký không khớp"},
		CreatedBy:     "user-20", CreatedAt: global_cur_time.Add(-144 * time.Hour), UpdatedAt: global_cur_time.Add(-72 * time.Hour),
	},
}
