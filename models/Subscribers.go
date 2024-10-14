package models

import (
	"gorm.io/gorm"
)

type Subscribers struct {
	gorm.Model
	ChatId int64
}
