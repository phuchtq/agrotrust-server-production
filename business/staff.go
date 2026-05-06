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
	"raise-child/model/dtos/request"
	"raise-child/model/dtos/response"
	"raise-child/model/entities"
	"raise-child/util"
	"raise-child/util/cache"
	on_chain "raise-child/util/on_chain"
	"sort"
	"strings"
	"time"

	"github.com/block-vision/sui-go-sdk/constant"
	"github.com/block-vision/sui-go-sdk/sui"
)

type staffService struct {
	redisCache cache.IRedisCache
	clients    map[string]sui.ISuiAPI
	errLogger  *log.Logger
}

func initializeStaffService(clients map[string]sui.ISuiAPI, errLogger *log.Logger) business.IStaffService {
	return &staffService{
		redisCache: cache.InitializeRedisCache(),
		clients:    clients,
		errLogger:  errLogger,
	}
}

func GenerateStaffService() (business.IStaffService, error) {
	return initializeStaffService(_networkAliases, util.GetLogConfig(shared.ERROR_LEVEL)), nil
}

const (
	staff_records_limit int = 10
)

// GetStaffByOwnerWallet implements business.IStaffService.
func (s *staffService) GetStaffByOwnerWallet(id string, ctx context.Context) (response.StaffResponse, error) {
	if !util.IsValidSuiAddressStrict(id) {
		return response.StaffResponse{}, errors.New(noti.GENERIC_ERROR_WARN_MSG)
	}

	var client = s.clients[constant.SuiTestnet]
	manage, err := on_chain.GetOnChainObject[entities.Manage](on_chain.GetOnChainObjectRequest{
		Client:    client,
		ObjectId:  os.Getenv(env.MANAGE_OBJECT_ID),
		ErrLogger: s.errLogger,
	}, ctx)
	if err != nil {
		return response.StaffResponse{}, err
	}

	var internalErr error = errors.New(noti.INTERNALL_ERR_MSG)
	if manage == nil {
		return response.StaffResponse{}, internalErr
	}

	var searchLen int
	if len(manage.VolunteerIds) > len(manage.LocalLeaderIds) {
		searchLen = len(manage.VolunteerIds)
	} else {
		searchLen = len(manage.LocalLeaderIds)
	}

	var staffId string
	for i := 0; i < searchLen; i++ {
		if i < len(manage.VolunteerIds) {
			if id == manage.VolunteerIds[i] {
				staffId = manage.VolunteerNfts[i]
				break
			}
		}

		if i < len(manage.LocalLeaderIds) {
			if id == manage.LocalLeaderIds[i] {
				staffId = manage.LocalLeaderNfts[i]
				break
			}
		}
	}

	if staffId == "" {
		return response.StaffResponse{}, nil
	}

	staff, err := getOnChainObject[entities.Staff](client, staffId, s.errLogger, ctx)
	if err != nil {
		return response.StaffResponse{}, err
	}

	if staff == nil {
		return response.StaffResponse{}, internalErr
	}

	return staff.ToStaffResponse(), nil
}

// GetStaff implements business.IStaffService.
func (s *staffService) GetStaff(id string, ctx context.Context) (response.StaffResponse, error) {
	if !util.IsValidSuiAddressStrict(id) {
		return response.StaffResponse{}, errors.New(noti.GENERIC_ERROR_WARN_MSG)
	}

	var client = s.clients[constant.SuiTestnet]
	staff, err := getOnChainObject[entities.Staff](client, id, s.errLogger, ctx)
	if err != nil {
		return response.StaffResponse{}, err
	}

	var res response.StaffResponse = staff.ToStaffResponse()

	// var module = on_chain.InitializeModuleStaff()
	// if nfts, _ := on_chain.GetOnChainOwnedObjects[entities.StaffNft](on_chain.GetOnChainOwnedObjectsRequest{
	// 	Client:       client,
	// 	OwnerAddress: staff.User,
	// 	StructType:   fmt.Sprintf("%s::%s::%s", os.Getenv(env.PACKAGE_ID), module.GetModule(), module.GetStaffNftObjectStruct()),
	// 	ErrLogger:    s.errLogger,
	// }, ctx); nfts != nil {
	// 	var nftsRes []response.StaffNftResponse
	// 	for _, nft := range nfts {
	// 		nftsRes = append(nftsRes, nft.ToStaffNftResponse())
	// 	}

	// 	res.Nfts = nftsRes
	// }

	return res, nil

	/////////////////
	// // MOCK DATA

	// for _, staff := range mockStaffs {
	// 	if staff.ID == id {
	// 		return response.StaffResponse{
	// 			ID:           id,
	// 			Role:         staff.Role,
	// 			IdentityCode: staff.IdentityCode,
	// 			User:         staff.Owner,
	// 			AvatarBlobID: staff.AvatarBlobID,
	// 			Region:       staff.Region,
	// 			FirstName:    staff.FirstName,
	// 			LastName:     staff.LastName,
	// 			Gender:       staff.Gender,
	// 			DateOfBirth:  util.TimeToRawDate(staff.DateOfBirth),
	// 			PhoneNumber:  staff.PhoneNumber,
	// 			Email:        staff.Email,
	// 			UploadedAt:   staff.UploadedAt,
	// 		}, nil
	// 	}
	// }
	// return response.StaffResponse{}, nil
}

// GetStaffs implements business.IStaffService.
func (s *staffService) GetStaffs(req request.GetStaffsRequest, ctx context.Context) (response.PaginationDataResponse, error) {
	req.Keyword = util.StandardizeString(req.Keyword)
	req.Region = util.StandardizeString(req.Region)
	req.SortOrder = util.StandardizeSortOrder(req.SortOrder)
	if req.Page < 1 {
		req.Page = 1
	}

	if req.PageSize < 1 {
		req.PageSize = default_page_size
	}

	var res response.PaginationDataResponse
	var redisKey string = s.getGetStaffsRedisKey(req)
	if s.redisCache.Get(redisKey, &res, ctx) {
		return res, nil
	}

	var client = s.clients[constant.SuiTestnet]
	var manageObj entities.Manage
	if !s.redisCache.Get(manageObj.GetRedisKey(), &manageObj, ctx) {
		res, err := on_chain.GetOnChainObject[entities.Manage](on_chain.GetOnChainObjectRequest{
			Client:    s.clients[constant.SuiTestnet],
			ObjectId:  os.Getenv(env.MANAGE_OBJECT_ID),
			ErrLogger: s.errLogger,
		}, ctx)
		if err != nil {
			return response.PaginationDataResponse{}, err
		}

		if res != nil {
			s.redisCache.Set(manageObj.GetRedisKey(), res, time.Minute, ctx)
			manageObj = *res
		}
	}

	staffs, err := on_chain.GetOnChainObjects[entities.StaffNft](on_chain.GetOnChainObjectsRequest{
		Client:    client,
		ObjectIds: manageObj.ChildIds,
		ErrLogger: s.errLogger,
	}, ctx)

	if err != nil {
		return response.PaginationDataResponse{}, err
	}

	if staffs == nil {
		return response.PaginationDataResponse{}, nil
	}

	var filteredStaffs []entities.StaffNft
	for i := len(staffs) - 1; i >= 0; i-- {
		var staff entities.StaffNft = staffs[i]

		if req.Role != "" {
			if util.StandardizeString(staff.Role) != util.StandardizeString(req.Role) { // Not matched
				continue
			}
		}

		if req.Region != "" {
			if util.StandardizeString(staff.Region) != req.Region { // Not matched
				continue
			}
		}

		if req.Gender != "" {
			if staff.Gender != req.Gender { // Not matched
				continue
			}
		}

		if req.YearOfBirth != nil {
			var dob time.Time = util.RawDateToTime(staff.DateOfBirth)
			if dob.Year() != *req.YearOfBirth { // Not matched
				continue
			}
		}

		if req.Keyword != "" {
			var firstName string = util.StandardizeString(staff.FirstName)
			var lastName string = util.StandardizeString(staff.LastName)
			if !strings.Contains(firstName, req.Keyword) && !strings.Contains(lastName, req.Keyword) && !strings.Contains(staff.IdentityCode, req.Keyword) && !strings.Contains(staff.PhoneNumber, req.Keyword) && !strings.Contains(staff.Email, req.Keyword) { // Not matched
				continue
			}
		}

		filteredStaffs = append(filteredStaffs, staff)
	}

	sort.Slice(filteredStaffs, func(i, j int) bool {
		var name1 string = filteredStaffs[i].LastName + " " + filteredStaffs[i].FirstName
		var name2 string = filteredStaffs[j].LastName + " " + filteredStaffs[j].FirstName

		if req.SortOrder == "ASC" {
			return name1 < name2
		}

		return name2 > name1
	})

	var skippedRecords int = (req.Page - 1) * req.PageSize
	if len(filteredStaffs) <= skippedRecords {
		return response.PaginationDataResponse{}, err
	}

	var data []response.StaffNftResponse
	for i := skippedRecords; i < len(filteredStaffs); i++ {
		data = append(data, filteredStaffs[i].ToStaffNftResponse())
		if len(data) == req.PageSize {
			break
		}
	}

	res = response.PaginationDataResponse{
		Data:       data,
		Amount:     len(data),
		Page:       req.Page,
		TotalPages: int(math.Ceil(float64(len(filteredStaffs)) / float64(req.PageSize))),
	}

	s.redisCache.Set(redisKey, res, time.Minute*5, ctx)

	return res, nil

	// //////////////////////////
	// // MOCK DATA
	// req.Keyword = util.StandardizeString(req.Keyword)
	// req.Region = util.StandardizeString(req.Region)
	// req.SortOrder = util.StandardizeSortOrder(req.SortOrder)
	// if req.Page < 1 {
	// 	req.Page = 1
	// }

	// if req.PageSize < 1 {
	// 	req.PageSize = default_page_size
	// }

	// var res response.PaginationDataResponse
	// var redisKey string = s.getGetStaffsRedisKey(req)
	// if s.redisCache.Get(redisKey, &res, ctx) {
	// 	return res, nil
	// }

	// var staffs = mockStaffs
	// var filteredStaffs []response.StaffNftResponse
	// for i := len(staffs) - 1; i >= 0; i-- {
	// 	var staff response.StaffNftResponse = staffs[i]

	// 	if req.Role != "" {
	// 		if util.StandardizeString(staff.Role) != util.StandardizeString(req.Role) { // Not matched
	// 			continue
	// 		}
	// 	}

	// 	if req.Region != "" {
	// 		if util.StandardizeString(staff.Region) != req.Region { // Not matched
	// 			continue
	// 		}
	// 	}

	// 	if req.Gender != "" {
	// 		if staff.Gender != req.Gender { // Not matched
	// 			continue
	// 		}
	// 	}

	// 	if req.YearOfBirth != nil {
	// 		if staff.DateOfBirth.Year() != *req.YearOfBirth { // Not matched
	// 			continue
	// 		}
	// 	}

	// 	if req.Keyword != "" {
	// 		var firstName string = util.StandardizeString(staff.FirstName)
	// 		var lastName string = util.StandardizeString(staff.LastName)
	// 		if !strings.Contains(firstName, req.Keyword) && !strings.Contains(lastName, req.Keyword) && !strings.Contains(staff.IdentityCode, req.Keyword) && !strings.Contains(staff.PhoneNumber, req.Keyword) && !strings.Contains(staff.Email, req.Keyword) { // Not matched
	// 			continue
	// 		}
	// 	}

	// 	filteredStaffs = append(filteredStaffs, staff)
	// }

	// sort.Slice(filteredStaffs, func(i, j int) bool {
	// 	var name1 string = filteredStaffs[i].LastName + " " + filteredStaffs[i].FirstName
	// 	var name2 string = filteredStaffs[j].LastName + " " + filteredStaffs[j].FirstName

	// 	if req.SortOrder == "ASC" {
	// 		return name1 < name2
	// 	}

	// 	return name2 > name1
	// })

	// var skippedRecords int = (req.Page - 1) * req.PageSize
	// if len(filteredStaffs) <= skippedRecords {
	// 	return response.PaginationDataResponse{}, nil
	// }

	// var data []response.StaffNftResponse
	// for i := skippedRecords; i < len(filteredStaffs); i++ {
	// 	data = append(data, filteredStaffs[i])
	// 	if len(data) == req.PageSize {
	// 		break
	// 	}
	// }

	// res = response.PaginationDataResponse{
	// 	Data:       data,
	// 	Amount:     len(data),
	// 	Page:       req.Page,
	// 	TotalPages: int(math.Ceil(float64(len(filteredStaffs)) / float64(req.PageSize))),
	// }

	// s.redisCache.Set(redisKey, res, time.Minute*5, ctx)

	// return res, nil
}

// GetStaffsV2 implements business.IStaffService.
func (s *staffService) GetStaffsV2(req request.GetStaffsRequest, ctx context.Context) (response.PaginationDataResponse, error) {
	panic("unimplemented")
}

func (s *staffService) getGetStaffsRedisKey(req request.GetStaffsRequest) string {
	var keyword string = "empty"
	if req.Keyword != "" {
		keyword = req.Keyword
	}

	var role string = "empty"
	if req.Role != "" {
		role = req.Role
	}

	var region string = "empty"
	if req.Region != "" {
		region = req.Region
	}

	var gender string = "empty"
	if req.Gender != "" {
		gender = req.Gender
	}

	var yob string = "empty"
	if req.YearOfBirth != nil {
		yob = fmt.Sprintf("%d", *req.YearOfBirth)
	}

	return fmt.Sprintf("staff:kw:%s:role:%s:region:%s:g:%s:y:%s:o:%s:s:%d:p:%d",
		keyword, role, region, gender, yob, req.SortOrder, req.PageSize, req.Page)
}

var mockStaffs = getMockStaffs()

func getMockStaffs() []response.StaffNftResponse {
	staffList := []response.StaffNftResponse{
		{
			ID: "1", Role: "Admin", IdentityCode: "ID001", IdentityCardBlobID: "blob-id-1",
			AvatarBlobID: "avatar-1", Region: "Hanoi", FirstName: "An", LastName: "Nguyen",
			Gender: "Male", DateOfBirth: time.Date(1990, 1, 15, 0, 0, 0, 0, time.UTC),
			PhoneNumber: "0901234567", Email: "an.nguyen@example.com", UploadedAt: time.Now(),
			Name: "Nguyen Van An", Url: "https://nft.example.com",
		},
		{
			ID: "2", Role: "Manager", IdentityCode: "ID002", IdentityCardBlobID: "blob-id-2",
			AvatarBlobID: "avatar-2", Region: "HCM", FirstName: "Binh", LastName: "Tran",
			Gender: "Female", DateOfBirth: time.Date(1992, 5, 20, 0, 0, 0, 0, time.UTC),
			PhoneNumber: "0902345678", Email: "binh.tran@example.com", UploadedAt: time.Now(),
			Name: "Tran Thi Binh", Url: "https://nft.example.com",
		},
		{
			ID: "3", Role: "Staff", IdentityCode: "ID003", IdentityCardBlobID: "blob-id-3",
			AvatarBlobID: "avatar-3", Region: "Danang", FirstName: "Cuong", LastName: "Le",
			Gender: "Male", DateOfBirth: time.Date(1995, 8, 10, 0, 0, 0, 0, time.UTC),
			PhoneNumber: "0903456789", Email: "cuong.le@example.com", UploadedAt: time.Now(),
			Name: "Le Van Cuong", Url: "https://nft.example.com",
		},
		{
			ID: "4", Role: "Staff", IdentityCode: "ID004", IdentityCardBlobID: "blob-id-4",
			AvatarBlobID: "avatar-4", Region: "Can Tho", FirstName: "Dung", LastName: "Pham",
			Gender: "Female", DateOfBirth: time.Date(1988, 12, 5, 0, 0, 0, 0, time.UTC),
			PhoneNumber: "0904567890", Email: "dung.pham@example.com", UploadedAt: time.Now(),
			Name: "Pham My Dung", Url: "https://nft.example.com",
		},
		{
			ID: "5", Role: "Staff", IdentityCode: "ID005", IdentityCardBlobID: "blob-id-5",
			AvatarBlobID: "avatar-5", Region: "Hue", FirstName: "Em", LastName: "Hoang",
			Gender: "Male", DateOfBirth: time.Date(1993, 3, 25, 0, 0, 0, 0, time.UTC),
			PhoneNumber: "0905678901", Email: "em.hoang@example.com", UploadedAt: time.Now(),
			Name: "Hoang Van Em", Url: "https://nft.example.com",
		},
		{
			ID: "6", Role: "Lead", IdentityCode: "ID006", IdentityCardBlobID: "blob-id-6",
			AvatarBlobID: "avatar-6", Region: "Hanoi", FirstName: "Giang", LastName: "Vu",
			Gender: "Female", DateOfBirth: time.Date(1991, 11, 30, 0, 0, 0, 0, time.UTC),
			PhoneNumber: "0906789012", Email: "giang.vu@example.com", UploadedAt: time.Now(),
			Name: "Vu Thu Giang", Url: "https://nft.example.com",
		},
		{
			ID: "7", Role: "Staff", IdentityCode: "ID007", IdentityCardBlobID: "blob-id-7",
			AvatarBlobID: "avatar-7", Region: "HCM", FirstName: "Hoang", LastName: "Phan",
			Gender: "Male", DateOfBirth: time.Date(1994, 7, 12, 0, 0, 0, 0, time.UTC),
			PhoneNumber: "0907890123", Email: "hoang.phan@example.com", UploadedAt: time.Now(),
			Name: "Phan Minh Hoang", Url: "https://nft.example.com",
		},
		{
			ID: "8", Role: "Staff", IdentityCode: "ID008", IdentityCardBlobID: "blob-id-8",
			AvatarBlobID: "avatar-8", Region: "Hanoi", FirstName: "Ien", LastName: "Dang",
			Gender: "Female", DateOfBirth: time.Date(1996, 2, 14, 0, 0, 0, 0, time.UTC),
			PhoneNumber: "0908901234", Email: "ien.dang@example.com", UploadedAt: time.Now(),
			Name: "Dang Ngoc Ien", Url: "https://nft.example.com",
		},
		{
			ID: "9", Role: "Staff", IdentityCode: "ID009", IdentityCardBlobID: "blob-id-9",
			AvatarBlobID: "avatar-9", Region: "Vinh", FirstName: "Khanh", LastName: "Bui",
			Gender: "Male", DateOfBirth: time.Date(1989, 9, 21, 0, 0, 0, 0, time.UTC),
			PhoneNumber: "0909012345", Email: "khanh.bui@example.com", UploadedAt: time.Now(),
			Name: "Bui Duy Khanh", Url: "https://nft.example.com",
		},
		{
			ID: "10", Role: "Admin", IdentityCode: "ID010", IdentityCardBlobID: "blob-id-10",
			AvatarBlobID: "avatar-10", Region: "HCM", FirstName: "Lan", LastName: "Do",
			Gender: "Female", DateOfBirth: time.Date(1997, 4, 18, 0, 0, 0, 0, time.UTC),
			PhoneNumber: "0900123456", Email: "lan.do@example.com", UploadedAt: time.Now(),
			Name: "Do Phuong Lan", Url: "https://nft.example.com0",
		},
		{
			ID: "11", Role: "Staff", IdentityCode: "ID011", IdentityCardBlobID: "blob-id-11",
			AvatarBlobID: "avatar-11", Region: "Quang Ninh", FirstName: "Minh", LastName: "Trinh",
			Gender: "Male", DateOfBirth: time.Date(1993, 10, 5, 0, 0, 0, 0, time.UTC),
			PhoneNumber: "0911223344", Email: "minh.trinh@example.com", UploadedAt: time.Now(),
			Name: "Trinh Nhat Minh", Url: "https://nft.example.com1",
		},
		{
			ID: "12", Role: "Staff", IdentityCode: "ID012", IdentityCardBlobID: "blob-id-12",
			AvatarBlobID: "avatar-12", Region: "Hanoi", FirstName: "Nga", LastName: "Dinh",
			Gender: "Female", DateOfBirth: time.Date(1995, 6, 25, 0, 0, 0, 0, time.UTC),
			PhoneNumber: "0912223344", Email: "nga.dinh@example.com", UploadedAt: time.Now(),
			Name: "Dinh Thuy Nga", Url: "https://nft.example.com2",
		},
		{
			ID: "13", Role: "Manager", IdentityCode: "ID013", IdentityCardBlobID: "blob-id-13",
			AvatarBlobID: "avatar-13", Region: "HCM", FirstName: "Oanh", LastName: "Phung",
			Gender: "Female", DateOfBirth: time.Date(1991, 2, 28, 0, 0, 0, 0, time.UTC),
			PhoneNumber: "0913223344", Email: "oanh.phung@example.com", UploadedAt: time.Now(),
			Name: "Phung Kieu Oanh", Url: "https://nft.example.com3",
		},
		{
			ID: "14", Role: "Staff", IdentityCode: "ID014", IdentityCardBlobID: "blob-id-14",
			AvatarBlobID: "avatar-14", Region: "Danang", FirstName: "Phuc", LastName: "Vo",
			Gender: "Male", DateOfBirth: time.Date(1998, 1, 10, 0, 0, 0, 0, time.UTC),
			PhoneNumber: "0914223344", Email: "phuc.vo@example.com", UploadedAt: time.Now(),
			Name: "Vo Hong Phuc", Url: "https://nft.example.com4",
		},
		{
			ID: "15", Role: "Staff", IdentityCode: "ID015", IdentityCardBlobID: "blob-id-15",
			AvatarBlobID: "avatar-15", Region: "Hue", FirstName: "Quang", LastName: "Mai",
			Gender: "Male", DateOfBirth: time.Date(1990, 11, 22, 0, 0, 0, 0, time.UTC),
			PhoneNumber: "0915223344", Email: "quang.mai@example.com", UploadedAt: time.Now(),
			Name: "Mai Duc Quang", Url: "https://nft.example.com5",
		},
		{
			ID: "16", Role: "Lead", IdentityCode: "ID016", IdentityCardBlobID: "blob-id-16",
			AvatarBlobID: "avatar-16", Region: "HCM", FirstName: "Son", LastName: "Lam",
			Gender: "Male", DateOfBirth: time.Date(1987, 8, 14, 0, 0, 0, 0, time.UTC),
			PhoneNumber: "0916223344", Email: "son.lam@example.com", UploadedAt: time.Now(),
			Name: "Lam Tung Son", Url: "https://nft.example.com6",
		},
		{
			ID: "17", Role: "Staff", IdentityCode: "ID017", IdentityCardBlobID: "blob-id-17",
			AvatarBlobID: "avatar-17", Region: "Hanoi", FirstName: "Thao", LastName: "Kieu",
			Gender: "Female", DateOfBirth: time.Date(1996, 3, 19, 0, 0, 0, 0, time.UTC),
			PhoneNumber: "0917223344", Email: "thao.kieu@example.com", UploadedAt: time.Now(),
			Name: "Kieu Thu Thao", Url: "https://nft.example.com7",
		},
		{
			ID: "18", Role: "Staff", IdentityCode: "ID018", IdentityCardBlobID: "blob-id-18",
			AvatarBlobID: "avatar-18", Region: "Binh Duong", FirstName: "Uy", LastName: "Dao",
			Gender: "Male", DateOfBirth: time.Date(1994, 5, 30, 0, 0, 0, 0, time.UTC),
			PhoneNumber: "0918223344", Email: "uy.dao@example.com", UploadedAt: time.Now(),
			Name: "Dao Gia Uy", Url: "https://nft.example.com8",
		},
		{
			ID: "19", Role: "Staff", IdentityCode: "ID019", IdentityCardBlobID: "blob-id-19",
			AvatarBlobID: "avatar-19", Region: "Hanoi", FirstName: "Vy", LastName: "Nguyen",
			Gender: "Female", DateOfBirth: time.Date(1999, 7, 7, 0, 0, 0, 0, time.UTC),
			PhoneNumber: "0919223344", Email: "vy.nguyen@example.com", UploadedAt: time.Now(),
			Name: "Nguyen Ha Vy", Url: "https://nft.example.com9",
		},
		{
			ID: "20", Role: "Manager", IdentityCode: "ID020", IdentityCardBlobID: "blob-id-20",
			AvatarBlobID: "avatar-20", Region: "HCM", FirstName: "Xuan", LastName: "Phan",
			Gender: "Female", DateOfBirth: time.Date(1992, 10, 10, 0, 0, 0, 0, time.UTC),
			PhoneNumber: "0920223344", Email: "xuan.phan@example.com", UploadedAt: time.Now(),
			Name: "Phan Thanh Xuan", Url: "https://nft.example.com0",
		},
	}

	return staffList
}
