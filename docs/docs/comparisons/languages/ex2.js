export const Ex2Title = `Grant Access Based on Inherited Permissions`;

export const Ex2Intro = `
Staff roles are used to define permissions that a given
user has within an organization. In this example, we show how
permissions can be inherited from other roles based on
the organization's staff hierarchy. Rego's `
  + "[`graph.reachable`](https://www.openpolicyagent.org/docs/policy-reference/#builtin-graph-graphreachable) "
  + `built-in function not only
keeps the policy concise but is also safe from infinite recursion
caused by cyclic dependencies in the staff hierarchy.
`;

export const Ex2Rego = `package example

manages := {
    "manager": {"supervisor", "security"},
    "supervisor": {"assistant"},
    "security": set(),
    "assistant": set(),
}

dataset_permissions := {
    "manager": {"salaries"},
    "supervisor": {"rotas"},
    "security": {"cctv"},
    "assistant": {"product_prices"},
}

default allow := false

allow if {
    some inherited_role in graph.reachable(
      manages, {input.role}
    )
    input.dataset in dataset_permissions[inherited_role]
}
`;

export const Ex2Python = `manages = {
    "manager":   ["supervisor", "security"],
    "supervisor": ["assistant"],
    "assistant": [],
    "security": [],
}

dataset_permissions = {
    "manager": ["salaries"],
    "supervisor": ["rotas"],
    "security": ["cctv"],
    "assistant": ["product_prices"],
}

def reachable_roles_for_role(role):
    roles = [role]

    for r in manages[role]:
        roles.append(r)
        roles.extend(reachable_roles_for_role(r))

    return roles

def allow(role, dataset):
    inheritable_roles = reachable_roles_for_role(role)

    for r in inheritable_roles:
        if r in dataset_permissions:
            if dataset in dataset_permissions[r]:
                return True

    return False
`;

export const Ex2Java = `package com.example.app;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;

public class AuthorizationUtil {
    private static Map<String, List<String>> manages =
        Map.of("manager", List.of("supervisor", "security"),
            "supervisor", List.of("assistant"), "security",
            List.of(), "assistant", List.of());

    private static Map<String, List<String>>
        datasetPermissions = Map.of("manager",
            List.of("salaries"), "supervisor",
            List.of("rotas"), "security", List.of("cctv"),
            "assistant", List.of("product_prices"));

    private static List<String> reachableRolesForRole(String role) {
        List<String> roles = new ArrayList<>();
        roles.add(role);

        for (String r : manages.get(role)) {
          roles.add(r);
          roles.addAll(reachableRolesForRole(r));
        }

        return roles;
    }

    public static boolean allow(String role, String dataset) {
        List<String> inheritableRoles =
            reachableRolesForRole(role);

        for (String r : inheritableRoles) {
            if (datasetPermissions.containsKey(r)) {
                if (datasetPermissions.get(r).contains(dataset)) {
                    return true;
                }
            }
        }

        return false;
    }
}
`;

export const Ex2Go = `package main

import (
    "fmt"
    "slices"
)

var manages = map[string][]string{
    "manager":    {"supervisor", "security"},
    "supervisor": {"assistant"},
    "security":   {},
    "assistant":  {},
}

var datasetPermissions = map[string][]string{
    "manager":    {"salaries"},
    "supervisor": {"rotas"},
    "security":   {"cctv"},
    "assistant":  {"product_prices"},
}

func reachableRolesForRole(role string) []string {
    roles := []string{role}

    for _, r := range manages[role] {
        roles = append(roles, r)
        roles = append(
            roles,
            reachableRolesForRole(r)...,
        )
    }

    return roles
}

func allow(role, dataset string) (bool, error) {
    inheritableRoles := reachableRolesForRole(role)

    for _, r := range inheritableRoles {
        if datasets, ok := datasetPermissions[r]; ok {
            if slices.Contains(datasets, dataset) {
                return true, nil
            }
        }
    }

    return false, nil
}
`;
