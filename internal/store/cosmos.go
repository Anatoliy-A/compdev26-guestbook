package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
)

// Cosmos stores entries in Azure Cosmos DB for NoSQL using Microsoft Entra ID only.
// On AKS, DefaultAzureCredential picks up Workload Identity; locally it falls back to `az login`.
type Cosmos struct {
	container *azcosmos.ContainerClient
}

func NewCosmos(endpoint, database, container string) (*Cosmos, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("create Azure credential: %w", err)
	}
	client, err := azcosmos.NewClient(endpoint, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("create Cosmos client: %w", err)
	}
	c, err := client.NewContainer(database, container)
	if err != nil {
		return nil, fmt.Errorf("open Cosmos container: %w", err)
	}
	return &Cosmos{container: c}, nil
}

func (c *Cosmos) Add(ctx context.Context, e Entry) error {
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = c.container.CreateItem(ctx, azcosmos.NewPartitionKeyString(e.GuestbookID), body, nil)
	return err
}

func (c *Cosmos) List(ctx context.Context, guestbookID string, limit int) ([]Entry, error) {
	// limit comes from code, never from user input.
	query := fmt.Sprintf("SELECT TOP %d * FROM c ORDER BY c.createdAt DESC", limit)
	pager := c.container.NewQueryItemsPager(query, azcosmos.NewPartitionKeyString(guestbookID), nil)

	entries := make([]Entry, 0, limit)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range page.Items {
			var e Entry
			if err := json.Unmarshal(item, &e); err != nil {
				return nil, err
			}
			entries = append(entries, e)
		}
	}
	return entries, nil
}
