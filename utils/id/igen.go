package idgen

import (
	"strconv"

	"github.com/google/uuid"
	"github.com/sony/sonyflake"
)

type Generator interface {
	NextID() (string, error)
}

var Default Generator = &sfWrapper{
	Sonyflake: sonyflake.NewSonyflake(sonyflake.Settings{}),
}

type sfWrapper struct {
	*sonyflake.Sonyflake
}

func (g *sfWrapper) NextID() (string, error) {
	id, err := g.Sonyflake.NextID()

	return strconv.FormatUint(id, 10), err
}

type uuidWrapper struct {
}

func (g *uuidWrapper) NextID() (string, error) {
	u, err := uuid.NewRandom()
	return u.String(), err
}
