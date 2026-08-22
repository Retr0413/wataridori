# Separate GitOps repository

Copy `application-repo-deliver-digest.yml` to the application repository. The
GitHub App named by its variables must be installed on the GitOps repository.

Copy the four workflows from `../same-repository/` to the GitOps repository,
except `deliver-digest.yml`. Configure the repository variables and protected
`production` Environment described in `docs/github-actions.md`.

The application workflow can create a dev manifest PR, but it receives no GCP
credential. The GitOps repository owns dev apply, production proposal, and the
manual production apply entrypoint.
