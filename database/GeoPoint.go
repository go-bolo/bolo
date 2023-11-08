package database

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GeoPoint struct {
	X, Y float64
}

func (loc GeoPoint) GormDataType() string {
	return "geometry"
}

func (loc GeoPoint) GormValue(ctx context.Context, db *gorm.DB) clause.Expr {
	return clause.Expr{
		SQL:  "GeomFromText(?)",
		Vars: []interface{}{fmt.Sprintf("POINT(%f %f)", loc.X, loc.Y)},
	}
}

func (loc *GeoPoint) Scan(src interface{}) error {
	switch b := src.(type) {
	case []byte:
		if len(b) != 25 {
			return fmt.Errorf("expected []bytes with length 25, got %d", len(b))
		}
		var latitude float64
		var longitude float64
		buf := bytes.NewReader(b[9:17])
		err := binary.Read(buf, binary.LittleEndian, &latitude)
		if err != nil {
			return err
		}
		buf = bytes.NewReader(b[17:25])
		err = binary.Read(buf, binary.LittleEndian, &longitude)
		if err != nil {
			return err
		}

		loc.X = latitude
		loc.Y = longitude
	default:
		return fmt.Errorf("expected []byte for Location type, got  %T", src)
	}
	return nil
}
