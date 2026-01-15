package databases

import "gorm.io/gorm"

type Database interface {
	DB() *gorm.DB
}
