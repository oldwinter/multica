package metrics

func normalizeWikiScope(value string) string {
	switch value {
	case "workspace", "project", "user", "all":
		return value
	default:
		return "unknown"
	}
}

func normalizeWikiSearchResult(value string) string {
	switch value {
	case "hit", "empty":
		return value
	default:
		return "unknown"
	}
}

func normalizeWikiReviewDecision(value string) string {
	switch value {
	case "accepted", "rejected":
		return value
	default:
		return "unknown"
	}
}
