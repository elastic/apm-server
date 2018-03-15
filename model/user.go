package model

import (
	"errors"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type User struct {
	Id        *string
	Email     *string
	Username  *string
	IP        *string
	UserAgent *string
}

func (u *User) Decode(input interface{}) error {
	raw, ok := input.(map[string]interface{})
	if raw == nil {
		return nil
	}
	if !ok {
		return errors.New("Invalid type for user")
	}
	df := utility.DataFetcher{}
	u.Id = df.StringPtr(raw, "id")
	u.Email = df.StringPtr(raw, "email")
	u.Username = df.StringPtr(raw, "username")
	u.IP = df.StringPtr(raw, "ip")
	u.UserAgent = df.StringPtr(raw, "user_agent")
	return df.Err
}
func (u *User) Transform() common.MapStr {
	if u == nil {
		return nil
	}
	user := common.MapStr{}
	utility.Add(user, "id", u.Id)
	utility.Add(user, "email", u.Email)
	utility.Add(user, "username", u.Username)
	utility.Add(user, "ip", u.IP)
	utility.Add(user, "user_agent", u.UserAgent)
	return user
}
