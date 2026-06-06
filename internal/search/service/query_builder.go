package service

import (
	"strings"
)

type QueryBuilder struct {
	conditions []string

	args []any
}

func NewQueryBuilder() *QueryBuilder {

	return &QueryBuilder{}
}

func (b *QueryBuilder) Add(
	condition string,
	arg any,
) {

	b.conditions = append(
		b.conditions,
		condition,
	)

	b.args = append(
		b.args,
		arg,
	)
}

func (b *QueryBuilder) Build() (string, []any) {

	if len(b.conditions) == 0 {
		return "", b.args
	}

	return "WHERE " +
			strings.Join(
				b.conditions,
				" AND ",
			),
		b.args
}
