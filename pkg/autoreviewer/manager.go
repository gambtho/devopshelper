package autoreviewer

import (
	"context"
	"time"

	adocore "github.com/microsoft/azure-devops-go-api/azuredevops/core"
	adogit "github.com/microsoft/azure-devops-go-api/azuredevops/git"
	adoidentity "github.com/microsoft/azure-devops-go-api/azuredevops/identity"
	"github.com/samkreter/go-core/log"

	"github.com/samkreter/devopshelper/pkg/store"
	"github.com/samkreter/devopshelper/pkg/types"
)

var (
	DefaultReconcilePeriod = time.Hour * 24 * 1
)

type Manager struct {
	AutoReviewers []*AutoReviewer
	repoStore     store.RepositoryStore
}

func NewDefaultManager(ctx context.Context, repoStore store.RepositoryStore,
	reviewerStore store.ReviewerStore, teamStore store.TeamStore,
	adoGitClient adogit.Client, aodIdentityClient adoidentity.Client,
	adoCoreClient adocore.Client) (*Manager, error) {
	repos, err := repoStore.GetAllRepositories(ctx)
	if err != nil {
		return nil, err
	}

	enabledRepos := []*types.Repository{}
	for _, repo := range repos {
		if repo.Enabled {
			enabledRepos = append(enabledRepos, repo)
		}
	}

	aReviewers := make([]*AutoReviewer, 0, len(repos))
	for _, repo := range enabledRepos {
		aReviewer, err := NewAutoReviewer(adoGitClient, aodIdentityClient, adoCoreClient, defaultBotIdentifier,
			repo, repoStore, reviewerStore, teamStore, Options{})
		if err != nil {
			return nil, err
		}

		aReviewers = append(aReviewers, aReviewer)
	}

	return &Manager{
		repoStore:     repoStore,
		AutoReviewers: aReviewers,
	}, nil
}

func (m *Manager) Run(ctx context.Context) error {
	logger := log.G(ctx)

	for _, aReviewer := range m.AutoReviewers {
		if aReviewer.Repo.LastReconciled.Add(DefaultReconcilePeriod).Before(time.Now()) {
			logger.Infof("reconciling repo: %s......", aReviewer.Repo.Name)
			if err := aReviewer.Reconcile(ctx); err != nil {
				return err
			}
			logger.Infof("Successfully reconciled repo: %s", aReviewer.Repo.Name)
		}

		logger.Infof("Starting Reviewer for repo: %s/%s", aReviewer.Repo.ProjectName, aReviewer.Repo.Name)
		if err := aReviewer.Run(ctx); err != nil {
			return err
		}
		logger.Infof("Finished Balancing Cycle for: %s/%s", aReviewer.Repo.ProjectName, aReviewer.Repo.Name)
	}
	return nil
}
