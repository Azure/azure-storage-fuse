// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License. See License.txt in the project root for license information.
package azstorage

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-storage-fuse/v2/internal"
)

type layoutResp struct {
	layout        *internal.Layout
	contentLength int64
	contentMD5    []byte
	lmt           *time.Time
	crtime        *time.Time
	metadata      map[string]*string
	eTag          *azcore.ETag
}

// getLayout gets the layout of the blob.
func getLayout(ctx context.Context, pager *runtime.Pager[blob.GetLayoutResponse]) (*layoutResp, error) {
	layoutRanges := make([]internal.LayoutRange, 0)

	var contentLength int64
	var eTag *azcore.ETag
	var lr *layoutResp

	for pager.More() {
		resp, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		if resp.BlobContentLength == nil {
			return nil, errors.New("failed to get layout: BlobContentLength is nil")
		}
		contentLength = *resp.BlobContentLength
		if eTag == nil {
			eTag = resp.ETag
		}

		if lr == nil {
			lr = &layoutResp{
				contentLength: contentLength,
				contentMD5:    resp.BlobContentMD5,
				lmt:           resp.LastModified,
				crtime:        resp.BlobCreationTime,
				metadata:      resp.Metadata,
				eTag:          resp.ETag,
			}
		}

		if resp.Endpoints == nil || resp.Endpoints.Endpoint == nil || len(resp.Endpoints.Endpoint) == 0 ||
			resp.Ranges == nil || resp.Ranges.Range == nil || len(resp.Ranges.Range) == 0 {
			// No layout means we can download the whole blob from the primary endpoint.
			lr.layout = &internal.Layout{
				LayoutRanges: nil,
			}
			return lr, nil
		}
		endpoints := make([]string, len(resp.Endpoints.Endpoint))
		for _, ep := range resp.Endpoints.Endpoint {
			if ep.Index == nil || ep.Value == nil {
				return nil, errors.New("failed to get layout: endpoint index or value is nil")
			}
			idx := int(*ep.Index)
			if idx < 0 || idx >= len(endpoints) {
				return nil, errors.New("failed to get layout: endpoint index out of bounds")
			}
			endpoints[idx] = *ep.Value
		}
		for _, r := range resp.Ranges.Range {
			if r.Start == nil || r.End == nil || r.EndpointIndex == nil {
				return nil, errors.New("failed to get layout: range start, end, or endpoint index is nil")
			}
			epIdx := int(*r.EndpointIndex)
			if epIdx < 0 || epIdx >= len(endpoints) {
				return nil, errors.New("failed to get layout: range endpoint index out of bounds")
			}
			layoutRanges = append(layoutRanges, internal.LayoutRange{
				Start:    *r.Start,
				End:      *r.End,
				Endpoint: endpoints[epIdx],
			})
		}
	}

	if lr != nil {
		// Sort the layout ranges by start offset to make sure they are in order.
		sort.Slice(layoutRanges, func(i, j int) bool {
			return layoutRanges[i].Start < layoutRanges[j].Start
		})

		lr.layout = &internal.Layout{
			LayoutRanges: layoutRanges,
		}
	} else {
		// we didn't get any response, return error
		return nil, errors.New("failed to get layout: no response")
	}

	return lr, nil
}
