# PAM Policy: Deny root unless JIT approved
# Zero-Trust Privileged Access Control
# Enterprise Security Lab | Manjula Wickramasuriya

package pam.root_access

default allow = false

# Allow non-root users
allow {
    input.user != "root"
}

# Allow root only if JIT-approved and not expired
allow {
    input.user == "root"
    input.approval.status == "approved"
    input.approval.expires > time.now_ns()
}

# Deny message
deny[msg] {
    not allow
    msg := "Root access denied — requires JIT approval"
}

# Demo test cases
test_allow_non_root {
    allow with input as {"user": "alice"}
}

test_deny_root_no_approval {
    not allow with input as {"user": "root"}
}

test_allow_root_approved {
    allow with input as {
        "user": "root",
        "approval": {
            "status": "approved",
            "expires": time.now_ns() + 7200000000000  # +2h
        }
    }
}
