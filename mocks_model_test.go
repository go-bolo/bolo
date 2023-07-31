package bolo_test

import (
	"strconv"
	"time"

	bolo "github.com/go-bolo/bolo"
)

type URLModel struct {
	ID        uint64    `gorm:"primary_key;column:id;" json:"id" filter:"param:id;type:number"`
	Title     string    `gorm:"column:title;not null;" json:"title" filter:"param:title;type:string"`
	Path      string    `gorm:"column:path;type:text" json:"path" filter:"param:path;type:string"`
	CreatorID *string   `gorm:"column:creatorId;type:int(11)" json:"creatorId" filter:"param:creatorId;type:number"`
	CreatedAt time.Time `gorm:"column:createdAt;" json:"createdAt" filter:"param:createdAt"`
	UpdatedAt time.Time `gorm:"column:updatedAt;" json:"updatedAt" filter:"param:updatedAt"`
}

func (r *URLModel) TableName() string {
	return "urls"
}

func (r *URLModel) GetID() string {
	return strconv.FormatUint(r.ID, 10)
}

func (r *URLModel) LoadData(c *bolo.RequestContext) error {
	return nil
}

func (r *URLModel) LoadTeaserData(c *bolo.RequestContext) error {
	return nil
}

func (r *URLModel) Save(c *bolo.RequestContext) error {
	app := c.App
	var err error
	db := app.GetDB()

	r.UpdatedAt = app.GetClock().Now()

	if r.ID == 0 {
		r.CreatedAt = app.GetClock().Now()
		// create ....
		err = db.Create(&r).Error
		if err != nil {
			return err
		}
	} else {
		// update ...
		err = db.Save(&r).Error
		if err != nil {
			return err
		}
	}

	return nil
}

func FindOneURL(c *bolo.RequestContext, id string) (*URLModel, error) {
	db := c.App.GetDB()

	record := URLModel{}
	err := db.First(&record, id).Error
	return &record, err
}
