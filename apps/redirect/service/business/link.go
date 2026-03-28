package business

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"net/url"
	"strings"

	"github.com/antinvestor/service-files/apps/redirect/service/models"
	"github.com/antinvestor/service-files/apps/redirect/service/repository"
	"github.com/pitabwire/util"
)

type linkBusiness struct {
	linkRepo repository.LinkRepository
}

func (lb *linkBusiness) CreateLink(ctx context.Context, link *models.Link) (*models.Link, error) {
	log := util.Log(ctx)

	if err := validateDestinationURL(link.DestinationURL); err != nil {
		return nil, fmt.Errorf("create link: %w", err)
	}

	if link.Slug == "" {
		slug, err := generateSlug()
		if err != nil {
			return nil, fmt.Errorf("create link: generate slug: %w", err)
		}
		link.Slug = slug
	}

	if link.State == models.LinkStateUnspecified {
		link.State = models.LinkStateActive
	}

	if !link.ValidXID(link.ID) {
		link.GenID(ctx)
	}

	if err := lb.linkRepo.Create(ctx, link); err != nil {
		return nil, fmt.Errorf("create link: %w", err)
	}

	log.Info("link created", "link_id", link.GetID(), "slug", link.Slug, "affiliate_id", link.AffiliateID)
	return link, nil
}

func (lb *linkBusiness) GetLink(ctx context.Context, id string) (*models.Link, error) {
	link, err := lb.linkRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get link %s: %w", id, err)
	}
	return link, nil
}

func (lb *linkBusiness) GetLinkBySlug(ctx context.Context, slug string) (*models.Link, error) {
	link, err := lb.linkRepo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("get link by slug %s: %w", slug, err)
	}
	return link, nil
}

func (lb *linkBusiness) UpdateLink(ctx context.Context, id string, updates map[string]any) (*models.Link, error) {
	link, err := lb.linkRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("update link %s: %w", id, err)
	}

	if v, ok := updates["destination_url"].(string); ok && v != "" {
		if err := validateDestinationURL(v); err != nil {
			return nil, fmt.Errorf("update link: %w", err)
		}
		link.DestinationURL = v
	}
	if v, ok := updates["title"].(string); ok && v != "" {
		link.Title = v
	}
	if v, ok := updates["campaign"].(string); ok {
		link.Campaign = v
	}
	if v, ok := updates["source"].(string); ok {
		link.Source = v
	}
	if v, ok := updates["medium"].(string); ok {
		link.Medium = v
	}
	if v, ok := updates["content"].(string); ok {
		link.Content = v
	}
	if v, ok := updates["term"].(string); ok {
		link.Term = v
	}
	if v, ok := updates["state"].(models.LinkState); ok && v != models.LinkStateUnspecified {
		link.State = v
	}

	if _, err := lb.linkRepo.Update(ctx, link); err != nil {
		return nil, fmt.Errorf("update link %s: %w", id, err)
	}

	util.Log(ctx).Info("link updated", "link_id", id)
	return link, nil
}

func (lb *linkBusiness) DeleteLink(ctx context.Context, id string) error {
	if err := lb.linkRepo.SoftDelete(ctx, id); err != nil {
		return fmt.Errorf("delete link %s: %w", id, err)
	}
	util.Log(ctx).Info("link deleted", "link_id", id)
	return nil
}

func (lb *linkBusiness) ListLinks(ctx context.Context, query string, affiliateID string, campaign string, state models.LinkState, limit int, offset int) ([]models.Link, error) {
	links, err := lb.linkRepo.SearchLinks(ctx, query, affiliateID, campaign, state, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list links: %w", err)
	}
	return links, nil
}

// validateDestinationURL ensures the URL is a valid http or https URL.
func validateDestinationURL(rawURL string) error {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("invalid destination URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("invalid destination URL scheme %q: only http and https are allowed", parsed.Scheme)
	}
	if parsed.Host == "" {
		return fmt.Errorf("invalid destination URL: host is required")
	}
	return nil
}

// generateSlug creates a short, URL-safe random slug (7 characters).
func generateSlug() (string, error) {
	b := make([]byte, 5)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random slug: %w", err)
	}
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b))[:7], nil
}
