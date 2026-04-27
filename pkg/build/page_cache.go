package build

import (
	"maps"
	"slices"

	"github.com/olimci/shizuka/pkg/transforms"
)

func cloneCachedPage(page *transforms.Page) *transforms.Page {
	if page == nil {
		return nil
	}

	cloned := *page
	cloned.QueryPage = nil
	cloned.Parent = nil
	cloned.Children = nil
	cloned.Assets = clonePageAssets(page.Assets, &cloned)
	cloned.Tags = slices.Clone(page.Tags)
	cloned.Aliases = slices.Clone(page.Aliases)
	cloned.Params = maps.Clone(page.Params)
	cloned.Queries = cloneCachedQueryResults(page.Queries)
	cloned.Headers = maps.Clone(page.Headers)
	cloned.Links = slices.Clone(page.Links)
	if page.Pagination != nil {
		pagination := *page.Pagination
		cloned.Pagination = &pagination
	}
	if page.Group != nil {
		group := *page.Group
		cloned.Group = &group
	}
	return &cloned
}

func clonePageForIndexCache(page *transforms.Page) *transforms.Page {
	cloned := cloneCachedPage(page)
	if cloned == nil {
		return nil
	}

	cloned.QueryPage = nil
	cloned.Parent = nil
	cloned.Children = nil
	cloned.Assets = make(map[string]*transforms.PageAsset)
	cloned.Body = ""
	cloned.Links = nil
	cloned.Queries = nil
	cloned.Pagination = nil
	cloned.Group = nil
	return cloned
}

func clonePageAssets(assets map[string]*transforms.PageAsset, owner *transforms.Page) map[string]*transforms.PageAsset {
	if assets == nil {
		return make(map[string]*transforms.PageAsset)
	}

	cloned := make(map[string]*transforms.PageAsset, len(assets))
	for key, asset := range assets {
		if asset == nil {
			continue
		}
		copied := *asset
		copied.Owner = owner
		cloned[key] = &copied
	}
	return cloned
}

func cloneCachedQueryResults(results map[string]*transforms.QueryResult) map[string]*transforms.QueryResult {
	if results == nil {
		return nil
	}

	cloned := make(map[string]*transforms.QueryResult, len(results))
	for key, result := range results {
		if result == nil {
			cloned[key] = nil
			continue
		}

		rows := make([]map[string]any, len(result.Rows))
		for i, row := range result.Rows {
			rows[i] = maps.Clone(row)
		}

		cloned[key] = &transforms.QueryResult{
			Rows:  rows,
			Pages: slices.Clone(result.Pages),
		}
	}
	return cloned
}
