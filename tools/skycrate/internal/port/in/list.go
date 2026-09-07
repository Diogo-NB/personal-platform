package in

import "context"

type ListRequest struct {
	Category string
	Tier     string
}

type Summary struct {
	ObjectCount int
	TotalBytes  int64
}

type Lister interface {
	List(ctx context.Context, request ListRequest) (Summary, error)
}
