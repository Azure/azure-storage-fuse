// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License. See License.txt in the project root for license information.
package azstorage

import (
	"context"
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

		contentLength = *resp.BlobContentLength
		if eTag == nil {
			eTag = resp.ETag
		}

		if lr == nil {
			lr = &layoutResp{
				contentLength: contentLength,
				contentMD5:    resp.BlobContentMD5,
				lmt:           resp.LastModified,
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
			endpoints[*ep.Index] = *ep.Value
		}
		for _, r := range resp.Ranges.Range {
			lr := internal.LayoutRange{
				Start:    *r.Start,
				End:      *r.End,
				Endpoint: endpoints[*r.EndpointIndex],
			}
			layoutRanges = append(layoutRanges, lr)
		}
	}

	lr.layout = &internal.Layout{
		LayoutRanges: layoutRanges,
	}

	return lr, nil
}
