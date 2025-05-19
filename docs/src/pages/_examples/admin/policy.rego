# Run your first Rego policy!
package payments

default allow := false

allow if {
	input.account.state == "open"
	input.user.risk_score in ["low", "medium"]
	input.transaction.amount <= 1000
}

# Open in the Rego Playground to see the full example.
