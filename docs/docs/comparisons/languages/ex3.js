export const Ex3Title = `Validate User-Generated Content`;

export const Ex3Intro = `
Policy is often used to guide users when they make mistakes.
Validation of user-generated resources can be complicated
and often needs to be implemented in many applications.
This example compares code for validating a user-submitted
blog post and shows how Rego rules can be defined
[incrementally](/docs/policy-language/#incremental-definitions).
`;

export const Ex3Rego = `package example

# ensure title and content are set
validations contains message if {
    some field in {"title", "content"}
    object.get(input, field, "") == ""
    message := sprintf("Value missing for field '%s'", [field])
}

# ensure title starts with a capital letter
validations contains "Title must start with a capital" if {
    not regex.match(\`^[A-Z]\`, input.title)
}

# ensure user identifier is set
validations contains "User email or id must be set" if {
    not input.user.email
    not input.user.id
}

# ensure example.com emails are not allowed
validations contains "example.com emails not allowed" if {
    endswith(input.user.email, "@example.com")
}
`;

export const Ex3Python = `def validations(input):
    messages = []

    # ensure title and content are set
    for field in ["title", "content"]:
        if not input.get(field):
            messages.append(f"Value missing for field '{field}'")

    # ensure title starts with a capital letter
    title = input.get("title", "")
    if title and not title[0].isupper():
        messages.append("Title must start with a capital")

    # ensure user identifier is set
    user = input.get("user", {})
    if not user:
        messages.append("User email or id must be set")
    else:
        if not any(k in user for k in ["email", "id"]):
            messages.append("User email or id must be set")
        # ensure example.com emails are not allowed
        if user.get("email", "").endswith("@example.com"):
            messages.append("example.com emails not allowed")

    return messages
`;

export const Ex3Java = `package com.example.app;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;

public class ValidationUtil {
private static List<String> allowedRoles = List.of("admin", "member", "viewer");

    public static List<String> validations(Map<String, Object> input) {
        List<String> messages = new ArrayList<>();

        // ensure title and content are set
        String[] fields = {"title", "content"};
        for (String field : fields) {
            if (!input.containsKey(field) || ((String) input.get(field)).isEmpty()) {
                messages.add("Value missing for field '" + field + "'");
            }
        }

        // ensure title starts with a capital letter
        String title = input.containsKey("title") ? (String) input.get("title") : "";
        if (!title.isEmpty() && !Character.isUpperCase(title.charAt(0))) {
            messages.add("Title must start with a capital");
        }

        // ensure user identifier is set
        if (!input.containsKey("user") || !(input.get("user") instanceof Map)) {
            messages.add("User email or id must be set");
        } else {
            Map<String, Object> user = (Map<String, Object>) input.get("user");
            boolean hasEmailOrId = user.containsKey("email") || user.containsKey("id");
            if (!hasEmailOrId) {
                messages.add("User email or id must be set");
            }
            // ensure example.com emails are not allowed
            if (user.containsKey("email") && ((String) user.get("email")).endsWith("@example.com")) {
                messages.add("example.com emails not allowed");
            }
        }

        return messages;
    }
}
`;

export const Ex3Go = `package main

import (
    "fmt"
    "strings"
)

func validations(input map[string]interface{}) []string {
    var messages []string

    // ensure that title and content are set
    fields := []string{"title", "content"}
    for _, field := range fields {
        if val, ok := input[field]; !ok || val == "" {
            messages = append(messages, fmt.Sprintf("Value missing for field '%s'", field))
        }
    }

    // ensure that title starts with a capital letter
    if title, ok := input["title"].(string); ok {
        if title != "" && title[0] < 'A' || title[0] > 'Z' {
            messages = append(messages, "Title must start with a capital")
        }
    }

    // ensure user identifier is set
    user, ok := input["user"].(map[string]interface{})
    if !ok {
        messages = append(messages, "User email or id must be set")
    } else {
        _, hasEmail := user["email"]
        _, hasId := user["id"]
        if !hasEmail && !hasId {
            messages = append(messages, "User email or id must be set")
        }
        if email, ok := user["email"].(string); ok {
            // ensure example.com emails are not allowed
            if strings.HasSuffix(email, "example.com") {
                messages = append(messages, "example.com emails not allowed")
            }
        }
    }

    return messages
}
`;
