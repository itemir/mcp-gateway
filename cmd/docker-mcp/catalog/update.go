package catalog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"
	
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
)

func resolveCatalogURL(ctx context.Context, name string, originalURL string) (string, error) {
	if name != DockerCatalogName {
		return originalURL, nil
	}

	userInfo, err := desktop.GetLoggedInUserInfo(ctx)
	if err != nil {
		return DockerCatalogURL, nil
	}

	// Check if user has organizations
	if len(userInfo.Organizations) > 0 {
		// Use the first available organization
		// Requires some product logic to account for the case where the user
		// is a member of multiple organizations
		primaryOrg := userInfo.Organizations[0]
		// TODO: This needs to be a service call to get the catalog for the
		// organization
		// The following URL will give a 404, and it is expected for now
		// It is for demonstration purposes only
		orgCatalogURL := fmt.Sprintf(
			"https://desktop.docker.com/mcp/catalog/v2/catalog-%s.yaml",
			primaryOrg,
		)
		return orgCatalogURL, nil
	}

	return DockerCatalogURL, nil
}

func Update(ctx context.Context, args []string) error {
	cfg, err := ReadConfig()
	if err != nil {
		return err
	}
	var names []string
	if len(args) == 0 {
		names = getAllCatalogNames(*cfg)
	}
	for _, arg := range args {
		if _, ok := cfg.Catalogs[arg]; ok {
			names = append(names, arg)
		} else {
			return fmt.Errorf("unknown catalog %q", arg)
		}
	}
	var errs []error
	for _, name := range names {
		catalog, ok := cfg.Catalogs[name]
		if !ok {
			continue
		}
		if err := updateCatalog(ctx, name, catalog); err != nil {
			errs = append(errs, err)
		}
		fmt.Println("updated:", name)

	}
	return errors.Join(errs...)
}

func getAllCatalogNames(cfg Config) []string {
	var names []string
	for name := range cfg.Catalogs {
		names = append(names, name)
	}
	return names
}

func updateCatalog(ctx context.Context, name string, catalog Catalog) error {
	url := catalog.URL

	var (
		catalogContent []byte
		err            error
	)

	// For the docker catalog, resolve the appropriate URL based on auth
	if name == DockerCatalogName {
		resolvedURL, err := resolveCatalogURL(ctx, name, url)
		if err != nil {
			url = DockerCatalogURL
		} else {
			url = resolvedURL
		}
		
	}
	
	if isValidURL(url) {
		catalogContent, err = DownloadFile(ctx, url)
	} else {
		catalogContent, err = os.ReadFile(url)
	}
	if err != nil {
		return err
	}

	cfg, err := ReadConfig()
	if err != nil {
		return err
	}
	cfg.Catalogs[name] = Catalog{
		DisplayName: catalog.DisplayName,
		URL:         catalog.URL,
		LastUpdate:  time.Now().Format(time.RFC3339),
	}
	if err := WriteConfig(cfg); err != nil {
		return err
	}

	if err := WriteCatalogFile(name, catalogContent); err != nil {
		return fmt.Errorf(
			"failed to write catalog %q: %w",
			name,
			err,
		)
	}
	return nil
}
