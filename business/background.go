package business

import (
	"context"
	"fmt"
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
	backgroundChildrenWithdrawRepo i_repository.IBackgroundChildrenWithdrawProposalRequestRepository
	leaderNotiRepo                 i_repository.ILeaderNotiRepository
	registrationRequestRepo        i_repository.IRegistrationRequestRepository
	centerRequestRepo              i_repository.ICenterRequestRepository
	uploadChildRequestRepo         i_repository.IUploadChildRequestRepository
	clients                        map[string]sui.ISuiAPI
	errLogger                      *log.Logger
}

func initializeBackgroundService(
	backgroundChildrenWithdrawRepo i_repository.IBackgroundChildrenWithdrawProposalRequestRepository,
	leaderNotiRepo i_repository.ILeaderNotiRepository,
	registrationRequestRepo i_repository.IRegistrationRequestRepository,
	centerRequestRepo i_repository.ICenterRequestRepository,
	uploadChildRequestRepo i_repository.IUploadChildRequestRepository,
	clients map[string]sui.ISuiAPI,
	errLogger *log.Logger,
) business.IBackgroundService {
	return &backgroundService{
		backgroundChildrenWithdrawRepo: backgroundChildrenWithdrawRepo,
		registrationRequestRepo:        registrationRequestRepo,
		centerRequestRepo:              centerRequestRepo,
		uploadChildRequestRepo:         uploadChildRequestRepo,
		clients:                        clients,
		errLogger:                      errLogger,
	}
}

func GenerateBackgroundService() (business.IBackgroundService, error) {
	var errLogger = util.GetLogConfig(shared.ERROR_LEVEL)

	cnn, err := db.ConnectDB(errLogger, db.InitializePostgreSQL())
	if err != nil {
		return nil, err
	}

	return initializeBackgroundService(
		repository.InitializeBackgroundChildrenWithdrawRequestRepository(cnn, errLogger),
		repository.InitializeLeaderNotiRepository(cnn, errLogger),
		repository.InitializeRegistrationRequestRepo(cnn, errLogger),
		repository.InitializeCenterRequestRepository(cnn, errLogger),
		repository.InitializeUploadChildRequestRepo(cnn, errLogger),
		_networkAliases,
		errLogger,
	), nil
}

// ProcessCreateChildrenWithdrawProposals implements business.IBackgroundService.
func (b *backgroundService) ProcessCreateChildrenWithdrawProposals(ctx context.Context) {
	var curTime time.Time = time.Now()
	if !isChildrenWithdrawProposalDateValid(curTime) || curTime.Hour() != 23 {
		return
	}

	reqs, _ := b.backgroundChildrenWithdrawRepo.GetCurrentPendingRequests(ctx)
	if reqs == nil {
		return
	}

	var client = b.clients[constant.SuiTestnet]
	manage, _ := on_chain.GetOnChainObject[entities.Manage](on_chain.GetOnChainObjectRequest{
		Client:    client,
		ObjectId:  os.Getenv(env.MANAGE_OBJECT_ID),
		ErrLogger: b.errLogger,
	}, ctx)
	if manage == nil {
		return
	}

	pool, _ := on_chain.GetOnChainObject[entities.MainPool](on_chain.GetOnChainObjectRequest{
		Client:    client,
		ObjectId:  os.Getenv(env.POOL_ID),
		ErrLogger: b.errLogger,
	}, ctx)
	if pool == nil {
		return
	}

	localPools, _ := on_chain.GetOnChainObjects[entities.LocalPool](on_chain.GetOnChainObjectsRequest{
		Client:    client,
		ObjectIds: pool.LocalPools,
		ErrLogger: b.errLogger,
	}, ctx)
	if localPools == nil {
		return
	}

	booksNeedWithdrawDates, _ := on_chain.GetOnChainObject[entities.BooksNeedWithdrawDates](on_chain.GetOnChainObjectRequest{
		Client:    client,
		ObjectId:  os.Getenv(env.BOOKS_NEED_WITHDRAW_DATES_ID),
		ErrLogger: b.errLogger,
	}, ctx)
	if booksNeedWithdrawDates == nil {
		return
	}

	healthNeedWithdrawDate, _ := on_chain.GetOnChainObject[entities.HealthInsuranceNeedWithdrawDate](on_chain.GetOnChainObjectRequest{
		Client:    client,
		ObjectId:  os.Getenv(env.HEALTH_INSURANCE_NEED_wITHDRAW_DATE_ID),
		ErrLogger: b.errLogger,
	}, ctx)
	if healthNeedWithdrawDate == nil {
		return
	}

	var modules []string
	var functions []string
	var args [][]interface{}
	var module = on_chain.InitializeModuleChild()
	for _, req := range reqs {
		var centerId string
		for i, region := range manage.LocalRegions {
			if region == req.Region {
				centerId = manage.ChildrenCenters[i]
				break
			}
		}

		center, _ := on_chain.GetOnChainObject[entities.Center](on_chain.GetOnChainObjectRequest{
			Client:    client,
			ObjectId:  centerId,
			ErrLogger: b.errLogger,
		}, ctx)
		if center == nil {
			return
		}

		children, _ := on_chain.GetOnChainObjects[entities.Child](on_chain.GetOnChainObjectsRequest{
			Client:    client,
			ObjectIds: center.ChildIDs,
			ErrLogger: b.errLogger,
		}, ctx)
		if children == nil {
			return
		}

		for _, child := range children {
			var localPoolId string
			for _, localPool := range localPools {
				if localPool.Region == req.Region {
					localPoolId = localPool.ID.ID
					break
				}
			}

			// Books Needs Withdraw
			for i := 0; i <= 1; i++ {
				if data := b.prepareDataCreateBooksNeedWithdrawProposal(
					child.BooksNeeds[i],
					req.ActorAddress,
					localPoolId,
					module,
					curTime,
					client,
					booksNeedWithdrawDates,
					ctx,
				); data != nil {
					modules = append(modules, module.GetModule())
					functions = append(functions, module.GetFunctionCreateChildBooksNeedWithdrawProposal())
					args = append(args, data)
				}
			}

			// Health Insurance Need Withdraw
			if data := b.prepareDataCreateHealthInsuranceNeedWithdrawProposal(
				child.HealthInsuranceNeed,
				req.ActorAddress,
				localPoolId,
				module,
				curTime,
				client,
				healthNeedWithdrawDate,
				ctx,
			); data != nil {
				modules = append(modules, module.GetModule())
				functions = append(functions, module.GetFunctionCreateChildHealthInsuranceNeedWithdrawProposal())
				args = append(args, data)
			}

			// Meal Need Withdraw
			if data := b.prepareDataCreateMealNeedWithdrawProposal(
				child.HealthInsuranceNeed,
				req.ActorAddress,
				localPoolId,
				module,
				curTime,
				client,
				ctx,
			); data != nil {
				modules = append(modules, module.GetModule())
				functions = append(functions, module.GetFunctionCreateChildMealNeedWithdrawProposal())
				args = append(args, data)
			}
		}

	}

	var errExecuteTxs error
	b.errLogger.Println("Create children withdraws call")
	for i := 1; i <= 3; i++ {
		if err := on_chain.BuildMultiBackgroundTransactions(on_chain.BuildMultiBackgroundTransactionsRequest{
			Client:    client,
			Modules:   modules,
			Functions: functions,
			Arguments: args,
			ErrLogger: b.errLogger,
		}, ctx); err == nil {
			break
		} else {
			errExecuteTxs = err
		}
	}

	if errExecuteTxs == nil {
		for i := 1; i <= 3; i++ {
			if b.backgroundChildrenWithdrawRepo.SetRequestsExecuted(reqs, ctx) == nil {
				return
			}
		}
	}
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

	b.errLogger.Println("Refund vote power call")
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
	pendingRes, err := b.registrationRequestRepo.GetPendingRequestsV2(ctx)
	if err != nil {
		return
	}

	if pendingRes != nil && len(pendingRes) > 0 {
		var refusedReqs, approvedReqs []entities.RegistrationRequest
		for _, req := range pendingRes {
			var rate float32 = float32(len(req.Approvers)) / float32(len(req.Approvers)+len(req.Refusers))
			if rate >= approve_rate_limit {
				approvedReqs = append(approvedReqs, req)
			} else {
				refusedReqs = append(refusedReqs, req)
			}
		}

		b.registrationRequestRepo.SetRefusedStatusesV2(refusedReqs, ctx)

		var modules []string
		var functions []string
		var args [][]interface{}
		var staffModule = on_chain.InitializeModuleStaff()
		var client = b.clients[constant.SuiTestnet]
		for _, req := range approvedReqs {
			modules = append(modules, staffModule.GetModule())

			switch req.RegisterRole {
			case admin_role:
				functions = append(functions, staffModule.GetFunctionRegisterAdmin())
				args = append(args, staffModule.ToRegisterAdminArguments(on_chain.RegisterAdminArguments{
					IdentityCode:       req.IdentityCode,
					IdentityCardBlobID: req.IdentityCardBlobID,
					AvatarBlobID:       req.AvatarBlobID,
					FirstName:          req.FirstName,
					LastName:           req.LastName,
					Gender:             req.Gender,
					DateOfBirth:        req.DateOfBirth,
					PhoneNumber:        req.PhoneNumber,
					Email:              req.Email,
					Owner:              req.CreatedBy,
				}))
			case local_leader_role:
				pool, _ := on_chain.GetOnChainObject[entities.MainPool](on_chain.GetOnChainObjectRequest{
					Client:    client,
					ObjectId:  os.Getenv(env.POOL_ID),
					ErrLogger: b.errLogger,
				}, ctx)

				if pool == nil {
					return
				}

				localPools, _ := on_chain.GetOnChainObjects[entities.LocalPool](on_chain.GetOnChainObjectsRequest{
					Client:    client,
					ObjectIds: pool.LocalPools,
					ErrLogger: b.errLogger,
				}, ctx)

				if localPools == nil || len(localPools) == 0 {
					return
				}

				var localPoolId string
				for _, localPool := range localPools {
					if localPool.Region == req.Region {
						localPoolId = localPool.ID.ID
						break
					}
				}

				if localPoolId == "" {
					localPoolId = os.Getenv(env.SHARED_LOCAL_POOL_ID)
				}

				functions = append(functions, staffModule.GetFunctionRegisterLeader())
				args = append(args, staffModule.ToRegisterNormalStaffArguments(on_chain.RegisterNormalStaffArguments{
					LocalPoolID: localPoolId,
					Region:      req.Region,
					RegisterAdminArguments: on_chain.RegisterAdminArguments{
						IdentityCode:       req.IdentityCode,
						IdentityCardBlobID: req.IdentityCardBlobID,
						AvatarBlobID:       req.AvatarBlobID,
						FirstName:          req.FirstName,
						LastName:           req.LastName,
						Gender:             req.Gender,
						DateOfBirth:        req.DateOfBirth,
						PhoneNumber:        req.PhoneNumber,
						Email:              req.Email,
						Owner:              req.CreatedBy,
					},
				}))
			case volunteer_role:
				functions = append(functions, staffModule.GetFunctionRegisterVolunteer())
				args = append(args, staffModule.ToRegisterNormalStaffArguments(on_chain.RegisterNormalStaffArguments{
					Region: req.Region,
					RegisterAdminArguments: on_chain.RegisterAdminArguments{
						IdentityCode:       req.IdentityCode,
						IdentityCardBlobID: req.IdentityCardBlobID,
						AvatarBlobID:       req.AvatarBlobID,
						FirstName:          req.FirstName,
						LastName:           req.LastName,
						Gender:             req.Gender,
						DateOfBirth:        req.DateOfBirth,
						PhoneNumber:        req.PhoneNumber,
						Email:              req.Email,
						Owner:              req.CreatedBy,
					},
				}))
			}
		}

		b.errLogger.Println("Registration Background call")
		if err := on_chain.BuildMultiBackgroundTransactions(on_chain.BuildMultiBackgroundTransactionsRequest{
			Client:    client,
			Modules:   modules,
			Functions: functions,
			Arguments: args,
			ErrLogger: b.errLogger,
		}, ctx); err != nil {
			return
		}

		b.registrationRequestRepo.SetApprovedStatusesV2(approvedReqs, ctx)
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

func (b *backgroundService) prepareDataCreateBooksNeedWithdrawProposal(needId, sender, poolId string, module on_chain.IModuleChild, curTime time.Time, client sui.ISuiAPI, withdrawDates *entities.BooksNeedWithdrawDates, ctx context.Context) []interface{} {
	need, _ := on_chain.GetOnChainObject[entities.BooksNeed](on_chain.GetOnChainObjectRequest{
		Client:    client,
		ObjectId:  needId,
		ErrLogger: b.errLogger,
	}, ctx)
	if need == nil {
		return nil
	}

	if len(need.Donations) == len(need.WithdrawsForNeed) {
		return nil
	}

	var rawExpectedWithdrawDate string
	if need.Semster == "1" {
		rawExpectedWithdrawDate = fmt.Sprintf("%s/%d", withdrawDates.FirstSemesterDate, curTime.Year())
	} else {
		rawExpectedWithdrawDate = fmt.Sprintf("%s/%d", withdrawDates.SecondSemesterDate, curTime.Year())
	}

	var expectedWithdrawDate time.Time = util.ToStartOfDate(util.RawDateToTime(rawExpectedWithdrawDate))
	if !expectedWithdrawDate.After(curTime) && expectedWithdrawDate.AddDate(0, 0, 7).Month() == curTime.Month() {
		var description string = fmt.Sprintf("Withdraw Books Need Semester %s - %d for child %s", need.Semster, curTime.Year(), util.FormatAddress(need.ChildID))
		return module.ToCreateChildNormalNeedWithdrawProposalArguments(on_chain.CreateChildNormalNeedWithdrawProposalArguments{
			NeedID:      needId,
			ChildID:     need.ChildID,
			LocalPool:   poolId,
			Description: description,
			ClosedAt:    util.ToMilliseconds(util.GetRequestDuration()),
			Sender:      sender,
		})
	}

	return nil
}

func (b *backgroundService) prepareDataCreateHealthInsuranceNeedWithdrawProposal(needId, sender, poolId string, module on_chain.IModuleChild, curTime time.Time, client sui.ISuiAPI, withdrawDate *entities.HealthInsuranceNeedWithdrawDate, ctx context.Context) []interface{} {
	need, _ := on_chain.GetOnChainObject[entities.HealthInsuranceNeed](on_chain.GetOnChainObjectRequest{
		Client:    client,
		ObjectId:  needId,
		ErrLogger: b.errLogger,
	}, ctx)
	if need == nil {
		return nil
	}

	if len(need.Donations) == len(need.WithdrawsForNeed) {
		return nil
	}

	var rawExpectedWithdrawDate string = fmt.Sprintf("%s/%d", withdrawDate.ExpectedDate, curTime.Year())
	var expectedWithdrawDate time.Time = util.ToStartOfDate(util.RawDateToTime(rawExpectedWithdrawDate))
	if !expectedWithdrawDate.After(curTime) && expectedWithdrawDate.AddDate(0, 0, 7).Month() == curTime.Month() {
		var description string = fmt.Sprintf("Withdraw Health Insurance Need %d for child %s", curTime.Year(), util.FormatAddress(need.ChildID))
		return module.ToCreateChildNormalNeedWithdrawProposalArguments(on_chain.CreateChildNormalNeedWithdrawProposalArguments{
			NeedID:      needId,
			ChildID:     need.ChildID,
			LocalPool:   poolId,
			Description: description,
			ClosedAt:    util.ToMilliseconds(util.GetRequestDuration()),
			Sender:      sender,
		})
	}

	return nil
}

func (b *backgroundService) prepareDataCreateMealNeedWithdrawProposal(needId, sender, poolId string, module on_chain.IModuleChild, curTime time.Time, client sui.ISuiAPI, ctx context.Context) []interface{} {
	need, _ := on_chain.GetOnChainObject[entities.MealNeed](on_chain.GetOnChainObjectRequest{
		Client:    client,
		ObjectId:  needId,
		ErrLogger: b.errLogger,
	}, ctx)
	if need == nil {
		return nil
	}

	totalSupportedMonths, _ := strconv.Atoi(need.TotalSupportedMonths)
	var expectedDuration int = totalSupportedMonths - len(need.WithdrawsForNeed)
	if expectedDuration == 0 {
		return nil
	}

	var previousDuration int = 0
	var expectedDate time.Time
	var foundIdx int = -1
	for i := len(need.Durations) - 1; i >= 0; i-- {
		var duration = need.Durations[0]
		var startPeriod time.Time = util.ToStartOfDate(util.RawDateToTime(duration.Fields.StartPeriod))
		var endPeriod time.Time = util.ToEndOfDate(util.RawDateToTime(duration.Fields.EndPeriod))
		var startMonth int = int(startPeriod.Month())
		var endMonth int = int(endPeriod.Month())
		if endMonth == 1 { // To next year
			endMonth = 13
		}

		var currentDuration int = endMonth - startMonth
		var totalDuration int = currentDuration + previousDuration
		var months int = totalDuration - expectedDuration
		if months >= 0 {
			var startDate = startPeriod.AddDate(0, months, 0)
			expectedDate = startDate.AddDate(0, 0, -3)
			foundIdx = i
			break
		}

		previousDuration = totalDuration
	}

	if !expectedDate.After(curTime) && expectedDate.AddDate(0, 0, 7).Month() == curTime.Month() {
		var description string = fmt.Sprintf("Withdraw Meal Need for child %s from %s to %s", util.FormatAddress(need.ChildID), need.Durations[foundIdx].Fields.StartPeriod, need.Durations[foundIdx].Fields.EndPeriod)
		return module.ToCreateChildNormalNeedWithdrawProposalArguments(on_chain.CreateChildNormalNeedWithdrawProposalArguments{
			NeedID:      needId,
			ChildID:     need.ChildID,
			LocalPool:   poolId,
			Description: description,
			ClosedAt:    util.ToMilliseconds(util.GetRequestDuration()),
			Sender:      sender,
		})
	}

	return nil
}
