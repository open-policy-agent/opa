package play

import future.keywords.and
import future.keywords.or

import data.groups

allow if input.user == "admin" or
    input.user in groups.admins

allow if input.action == "write" and
    input.user in groups.editors