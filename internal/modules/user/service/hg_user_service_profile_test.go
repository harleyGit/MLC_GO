package UserServicePackage

import (
	UserDtoPackage "MLC_GO/internal/modules/user/dto"
	"context"
	"errors"
	"testing"
)

func TestUpdateProfile_ValidateRequiredFields(t *testing.T) {
	svc := &UserService{}

	_, err := svc.UpdateProfile(context.Background(), "hgid_1001", &UserDtoPackage.HGUpdateUserProfileReqDTO{})
	if !errors.Is(err, ErrProfileNoField) {
		t.Fatalf("expected ErrProfileNoField, got err=%v", err)
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
	full, err := normalizeBirthDate("2020-08-16")
	if err != nil || full != "2020-08-16" {
		t.Fatalf("unexpected full date result, date=%s err=%v", full, err)
	}

	month, err := normalizeBirthDate("2020-08")
	if err != nil || month != "2020-08-01" {
		t.Fatalf("unexpected month date result, date=%s err=%v", month, err)
	}

	_, err = normalizeBirthDate("2020/08")
	if !errors.Is(err, ErrProfileBirthDateInvalid) {
		t.Fatalf("expected ErrProfileBirthDateInvalid, got err=%v", err)
	}
}
