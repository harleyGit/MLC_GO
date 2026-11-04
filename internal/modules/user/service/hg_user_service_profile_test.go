package UserServicePackage

import (
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	hg_time "MLC_GO/internal/pkg/hg_time"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestUpdateProfile_ValidateRequiredFields(t *testing.T) {
	svc := &UserService{}

	_, err := svc.UpdateProfile(context.Background(), "hgid_1001", &UserDtoPackage.HGUpdateUserProfileReqDTO{})
	if !errors.Is(err, ErrProfileNoField) {
		t.Fatalf("expected ErrProfileNoField, got err=%v", err)
	}
}

func TestUpdateProfile_RejectsOverlongSignature(t *testing.T) {
	svc := &UserService{}
	signature := strings.Repeat("签", 201)
	_, err := svc.UpdateProfile(context.Background(), "user-1", &UserDtoPackage.HGUpdateUserProfileReqDTO{Signature: &signature})
	if !errors.Is(err, ErrProfileSignatureTooLong) {
		t.Fatalf("签名超过 200 字符应被拒绝: %v", err)
	}
}

func TestUpdateProfile_ValidateGenderRange(t *testing.T) {
	svc := &UserService{}
	gender := 3

	_, err := svc.UpdateProfile(context.Background(), "hgid_1001", &UserDtoPackage.HGUpdateUserProfileReqDTO{
		Gender: &gender,
	})
	if !errors.Is(err, ErrProfileGenderInvalid) {
		t.Fatalf("expected ErrProfileGenderInvalid, got err=%v", err)
	}
}

func TestNormalizeBirthDate(t *testing.T) {
	full, err := hg_time.NormalizeBirthDate(hg_time.ClientTime{Value: "2020-08-16", Format: "date"})
	if err != nil || full != "2020-08-16" {
		t.Fatalf("unexpected full date result, date=%s err=%v", full, err)
	}

	month, err := hg_time.NormalizeBirthDate(hg_time.ClientTime{Value: "2020-08", Format: "year-month"})
	if err != nil || month != "2020-08-01" {
		t.Fatalf("unexpected month date result, date=%s err=%v", month, err)
	}

	_, err = hg_time.NormalizeBirthDate(hg_time.ClientTime{Value: "2020/08", Format: "year-month"})
	if err == nil {
		t.Fatalf("expected error for invalid year-month format")
	}
}
