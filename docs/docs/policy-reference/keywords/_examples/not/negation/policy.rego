package play

deny contains "must be staff" if {
	not "staff" in input.roles
}

deny contains "must be example.com account" if {
	not endswith(input.email, "@example.com")
}

deny contains "cannot be accesed over VPN" if {
	not input.is_vpn
}
