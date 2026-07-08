package play

matching_repos contains repo if {
	some repo, url in input.repos

	regex.match(`(?i)^github.com\/styrainc\/`, url)
}
