package user

import (
	"testing"

	"github.com/gin-gonic/gin/binding"
	"github.com/rowlys/panjang-umur-backend/internal/models"
)

func TestRegisterInputGender(t *testing.T) {
	cases := map[models.Gender]bool{
		models.GenderUnspecified: false,
		models.GenderMale:        true,
		models.GenderFemale:      true,
		3:                        false,
	}
	for gender, wantValid := range cases {
		input := RegisterInput{Username: "u", Name: "n", Password: "secret", Gender: gender}
		err := binding.Validator.ValidateStruct(input)
		if (err == nil) != wantValid {
			t.Errorf("gender %d: valid=%v, want %v (err: %v)", gender, err == nil, wantValid, err)
		}
	}
}

func TestUpdateProfileInputGender(t *testing.T) {
	gender := func(g models.Gender) *models.Gender { return &g }
	cases := []struct {
		gender    *models.Gender
		wantValid bool
	}{
		{nil, true},
		{gender(models.GenderMale), true},
		{gender(models.GenderFemale), true},
		{gender(models.GenderUnspecified), false},
	}
	for _, c := range cases {
		input := UpdateProfileInput{Name: "n", Username: "u", Gender: c.gender}
		err := binding.Validator.ValidateStruct(input)
		if (err == nil) != c.wantValid {
			t.Errorf("gender %v: valid=%v, want %v (err: %v)", c.gender, err == nil, c.wantValid, err)
		}
	}
}
