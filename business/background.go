package business

import (
	"context"
	"log"
	"os"
	"raise-child/constants/env"
	"raise-child/constants/shared"
	"raise-child/interfaces/business"
	i_repository "raise-child/interfaces/repository"
	"raise-child/model/entities"
	"raise-child/repository"
	"raise-child/util"
	"raise-child/util/db"
	on_chain "raise-child/util/on_chain"
	"strconv"
	"time"

	"github.com/block-vision/sui-go-sdk/constant"
	"github.com/block-vision/sui-go-sdk/sui"
)

type backgroundService struct {
	registrationRequestRepo i_repository.IRegistrationRequestRepository
	centerRequestRepo       i_repository.ICenterRequestRepository
	uploadChildRequestRepo  i_repository.IUploadChildRequestRepository
	clients                 map[string]sui.ISuiAPI
	errLogger               *log.Logger
}

func initializeBackgroundService(
	registrationRequestRepo i_repository.IRegistrationRequestRepository,
	centerRequestRepo i_repository.ICenterRequestRepository,
	uploadChildRequestRepo i_repository.IUploadChildRequestRepository,
	clients map[string]sui.ISuiAPI,
	errLogger *log.Logger,
) business.IBackgroundService {
	return &backgroundService{
		registrationRequestRepo: registrationRequestRepo,
		centerRequestRepo:       centerRequestRepo,
		uploadChildRequestRepo:  uploadChildRequestRepo,
		clients:                 clients,
		errLogger:               errLogger,
	}
}

func GenerateBackgroundService() (business.IBackgroundService, error) {
	var errLogger = util.GetLogConfig(shared.ERROR_LEVEL)

	cnn, err := db.ConnectDB(errLogger, db.InitializePostgreSQL())
	if err != nil {
		return nil, err
	}

	return initializeBackgroundService(
		repository.InitializeRegistrationRequestRepo(cnn, errLogger),
		repository.InitializeCenterRequestRepository(cnn, errLogger),
		repository.InitializeUploadChildRequestRepo(cnn, errLogger),
		_networkAliases,
		errLogger,
	), nil
}

// ProcessRefundVotePower implements business.IBackgroundService.
func (b *backgroundService) ProcessRefundVotePower(ctx context.Context) {
	var client = b.clients[constant.SuiTestnet]
	pool, _ := on_chain.GetOnChainObject[entities.MainPool](on_chain.GetOnChainObjectRequest{
		Client:    client,
		ObjectId:  os.Getenv(env.POOL_ID),
		ErrLogger: b.errLogger,
	}, ctx)
	if pool == nil {
		return
	}

	pendingProposals, _ := on_chain.GetOnChainObjects[entities.WithdrawProposal](on_chain.GetOnChainObjectsRequest{
		Client:    client,
		ObjectIds: pool.PendingWithdrawProposals,
		ErrLogger: b.errLogger,
	}, ctx)
	if pendingProposals == nil || len(pendingProposals) == 0 {
		return
	}

	var curTime time.Time = time.Now()
	var modules []string
	var functions []string
	var args [][]interface{}
	var module = on_chain.InitializeModuleRefund()
	for _, proposal := range pendingProposals {
		miliSecsClosedAt, _ := strconv.ParseInt(proposal.ClosedAt, 10, 64)
		if util.MilliSecToTime(miliSecsClosedAt).After(curTime) {
			continue
		}

		withdrawAmount, _ := strconv.ParseInt(proposal.WithdrawAmount, 10, 64)
		totalApproveWeight, _ := strconv.ParseInt(proposal.TotalApproveWeight, 10, 64)
		if totalApproveWeight >= withdrawAmount {
			continue
		}

		var targetId string
		switch proposal.Purpose {
		case string(entities.BOOKS_NEED_WITHDRAW_PROPOSAL_PURPOSE):
			functions = append(functions, module.GetFunctionRefundChildBooksNeedVotePower())
			targetId = proposal.TargetID
		case string(entities.MEAL_NEED_WITHDRAW_PROPOSAL_PURPOSE):
			functions = append(functions, module.GetFunctionRefundChildMealNeedVotePower())
			targetId = proposal.TargetID
		case string(entities.HEALTH_INSURANCE_NEED_WITHDRAW_PROPOSAL_PURPOSE):
			functions = append(functions, module.GetFunctionRefundChildHealthInsuranceNeedVotePower())
			targetId = proposal.TargetID
		case string(entities.SPECIAL_NEED_CAMPAIGN_WITHDRAW_PROPOSAL_PURPOSE):
			functions = append(functions, module.GetFunctionRefundChildSpecialNeedCampaignVotePower())
			targetId = proposal.TargetID
		case string(entities.POOL_CAMPAIGN_WITHDRAW_PROPOSAL_PURPOSE):
			functions = append(functions, module.GetFunctionRefundPoolCampaignVotePower())
			targetId = proposal.TargetID
		case string(entities.POOL_WITHDRAW_PROPOSAL_PURPOSE):
			functions = append(functions, module.GetFunctionRefundPoolVotePower())
			if proposal.IsFromLocalPool {
				targetId = proposal.TargetID
			} else {
				targetId = os.Getenv(env.SHARED_LOCAL_POOL_ID)
			}
		}

		modules = append(modules, module.GetModule())
		args = append(args, module.ToRefundVotePowerArguments(on_chain.RefundVotePowerArguments{
			TargetID:   targetId,
			ProposalID: proposal.ID.ID,
		}))
	}

	on_chain.BuildMultiBackgroundTransactions(on_chain.BuildMultiBackgroundTransactionsRequest{
		Client:    client,
		Modules:   modules,
		Functions: functions,
		Arguments: args,
		ErrLogger: b.errLogger,
	}, ctx)
}

// ProcessBackgroundCenterRequests implements business.IBackgroundService.
func (b *backgroundService) ProcessBackgroundCenterRequests(ctx context.Context) {
	pendingRes, approvedRes, err := b.centerRequestRepo.GetPendingRequests(ctx)
	if err != nil {
		return
	}

	if pendingRes != nil && len(pendingRes) > 0 {
		var refusedReqs []entities.BackgroundRecord
		for _, req := range pendingRes {
			var rate float32 = float32(len(req.Approvers)) / float32(len(req.Approvers)+len(req.Refusers))
			if rate >= approve_rate_limit {
				approvedRes = append(approvedRes, req)
			} else {
				refusedReqs = append(refusedReqs, req)
			}
		}

		b.centerRequestRepo.SetRefusedStatuses(refusedReqs, ctx)

		var modules []string
		var functions []string
		var args [][]interface{}
		var module = on_chain.InitializeModuleManage()
		for _, req := range approvedRes {
			// modules[i] = module.GetModule()
			// functions[i] = module.GetFunctionMintUploadCenterCap()
			// args = append(args, module.ToMintCapArguments(on_chain.MintCapArguments{
			// 	Recipient: req.Sender,
			// }))

			modules = append(modules, module.GetModule())
			functions = append(functions, module.GetFunctionMintUploadCenterCap())

			args = append(args, module.ToMintCapArguments(on_chain.MintCapArguments{
				Recipient: req.Sender,
			}))
		}

		if err := on_chain.BuildMultiBackgroundTransactions(on_chain.BuildMultiBackgroundTransactionsRequest{
			Client:    b.clients[constant.SuiTestnet],
			Modules:   modules,
			Functions: functions,
			Arguments: args,
			ErrLogger: b.errLogger,
		}, ctx); err != nil {
			return
		}

		b.centerRequestRepo.SetApprovedStatuses(approvedRes, ctx)
	}
}

// ProcessBackgroundRegistrationRequests implements business.IBackgroundService.
func (b *backgroundService) ProcessBackgroundRegistrationRequests(ctx context.Context) {
	pendingRes, approvedRes, err := b.registrationRequestRepo.GetPendingRequests(ctx)
	if err != nil {
		return
	}

	if pendingRes != nil && len(pendingRes) > 0 {

		var refusedReqs []entities.BackgroundRecord
		for _, req := range pendingRes {
			var rate float32 = float32(len(req.Approvers)) / float32(len(req.Approvers)+len(req.Refusers))
			if rate >= approve_rate_limit {
				approvedRes = append(approvedRes, req)
			} else {
				refusedReqs = append(refusedReqs, req)
			}
		}

		b.registrationRequestRepo.SetRefusedStatuses(refusedReqs, ctx)

		var modules []string
		var functions []string
		var args [][]interface{}
		var module = on_chain.InitializeModuleManage()
		for _, req := range approvedRes {
			// modules[i] = module.GetModule()
			// switch req.Role {
			// case admin_role:
			// 	functions[i] = module.GetFunctionMintRegisterAdminCap()
			// case local_leader_role:
			// 	functions[i] = module.GetFunctionMintRegisterLeaderCap()
			// case volunteer_role:
			// 	functions[i] = module.GetFunctionMintRegisterVolunteerCap()
			// }
			// args = append(args, module.ToMintCapArguments(on_chain.MintCapArguments{
			// 	Recipient: req.Sender,
			// }))

			// Thêm phần tử vào cuối slice
			modules = append(modules, module.GetModule())

			switch req.Role {
			case admin_role:
				functions = append(functions, module.GetFunctionMintRegisterAdminCap())
			case local_leader_role:
				functions = append(functions, module.GetFunctionMintRegisterLeaderCap())
			case volunteer_role:
				functions = append(functions, module.GetFunctionMintRegisterVolunteerCap())
			}

			args = append(args, module.ToMintCapArguments(on_chain.MintCapArguments{
				Recipient: req.Sender,
			}))
		}

		if err := on_chain.BuildMultiBackgroundTransactions(on_chain.BuildMultiBackgroundTransactionsRequest{
			Client:    b.clients[constant.SuiTestnet],
			Modules:   modules,
			Functions: functions,
			Arguments: args,
			ErrLogger: b.errLogger,
		}, ctx); err != nil {
			return
		}

		b.registrationRequestRepo.SetApprovedStatuses(approvedRes, ctx)
	}
}

// ProcessBackgroundUploadChildRequests implements business.IBackgroundService.
func (b *backgroundService) ProcessBackgroundUploadChildRequests(ctx context.Context) {
	// pendingRes, approvedRes, err := b.uploadChildRequestRepo.GetPendingRequests(ctx)
	// if err != nil {
	// 	return
	// }

	// var refusedReqs []entities.BackgroundRecord
	// for _, req := range pendingRes {
	// 	var rate float32 = float32(len(req.Approvers)) / float32(len(req.Approvers)+len(req.Refusers))
	// 	if rate >= approve_rate_limit {
	// 		approvedRes = append(approvedRes, req)
	// 	} else {
	// 		refusedReqs = append(refusedReqs, req)
	// 	}
	// }

	// b.uploadChildRequestRepo.SetRefusedStatuses(refusedReqs, ctx)

	// var modules []string
	// var functions []string
	// var args [][]interface{}
	// var module = on_chain.InitializeModuleManage()
	// for i, req := range approvedRes {
	// 	modules[i] = module.GetModule()
	// 	functions[i] = module.GetFunctionMintUploadCenterCap()
	// 	args = append(args, module.ToMintCapArguments(on_chain.MintCapArguments{
	// 		Recipient: req.Sender,
	// 	}))
	// }

	// if err := on_chain.BuildMultiBackgroundTransactions(on_chain.BuildMultiBackgroundTransactionsRequest{
	// 	Client:    b.clients[constant.SuiTestnet],
	// 	Modules:   modules,
	// 	Functions: functions,
	// 	Arguments: args,
	// 	ErrLogger: b.errLogger,
	// }, ctx); err != nil {
	// 	return
	// }

	// b.uploadChildRequestRepo.SetApprovedStatuses(approvedRes, ctx)
}
