package models

import (
	"bn-service/contracts"
	"gorm.io/gorm"
)

type Event struct {
	gorm.Model
	Subject string
	Payload contracts.User `gorm:"type:jsonb"`
}
