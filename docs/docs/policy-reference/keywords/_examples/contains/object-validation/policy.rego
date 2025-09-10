package play

import rego.v1

approval_min := 350

reasons[item.id] contains "category must be set" if {
	some item in input.items

	object.get(item, "category", "") == ""
}

reasons[item.id] contains message if {
	some item in input.items

	item.amount > approval_min

	object.get(item, "approved_by", "") == ""

	message := sprintf(
		"items over %d must be approved",
		[approval_min],
	)
}

reasons[item.id] contains "approver does not exist" if {
	some item in input.items

	not data.approvers[item.approved_by]
}
