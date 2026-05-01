package workspaces

const (
	errorServiceWorkspace = "workspace"
)

func errorContext(service, detail string) string {
	if service == "" {
		return detail
	}
	if detail == "" {
		return service
	}
	return service + ": " + detail
}
