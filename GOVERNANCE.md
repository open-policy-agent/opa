# Project Governance

This document defines the governance process for the open-policy-agent GitHub organization.

The MAINTAINERS.md file in this repository contains the list of OPA project maintainers and their "area of expertise". An area of expertise is defined as a set of repositories or repository subtrees.

## Voting

Maintainers use "organizational voting" to approve changes so that no single organization can dominate an area of expertise.

* "Organizations relevant to a change" are all those organizations with an area of expertise that covers the change.

* "Organizations with an area of expertise" are those organizations for which there is a maintainer from that organization with that area of expertise.

* Individuals not associated with or employed by a company or organization are allowed one organization vote.

* Each company or organization (regardless of the number of maintainers associated with or employed by that company/organization) receives one organization vote.  Any maintainer from an organization may cast the vote for that organization.

For example, consider the following scenario.

* Two maintainers are employed by Company X, two by Company Y, two by Company Z, and one maintainer is an unaffiliated individual

* Area of expertise E covers the repository R

* One maintainer from Company X, two from Company Y, and the un-affiliated individual all have expertise E

For any change requiring a vote to repository R, three "organization votes" are possible: one for X, one for Y, and one for the un-affiliated individual.

Unless specified otherwise, a vote passes when greater than fifty percent of the organization votes are in favour.

## Code Changes

All code changes should go through the Pull Request (PR) process. PRs should only be merged after receiving approval (via GitHub) from at least one other member of the GitHub team associated with the area(s) of expertise.

We do not vote formally on every code change, but we do expect that every code change merged has the same community support as if the change were approved by a formal vote. When a merge occurs without sufficient community support, the change should be reverted until the dispute is resolved through discussion. Any team member who feels that a technical decision cannot be reached can call for a formal vote following the rules outlined above in either the PR or a separate issue.

## Non-code Changes

Changes that are not PRs will be voted on through GitHub issues.  Maintainers should indicate their yes/no vote on that GitHub issue, and after a suitable period of time, the votes will be tallied and the outcome noted.

The following changes, while governed by the language above, require additional clarification.

### Changes in Maintainership

New maintainers for an area of expertise are proposed by an existing maintainer for that area of expertise and are elected by a 2/3 majority of the organizations with that area of expertise.

Maintainer status expires after 1 year but a request to self-renew can be made within 1 month of expiry.

Maintainers for an area of expertise can be removed by a 2/3 majority of the organizations with that area of expertise.

### Changes in Governance

All changes in Governance require a 2/3 majority organization vote from all areas of expertise.

### New Repositories

New repositories require a 2/3 majority organization vote from all areas of expertise.

## GitHub Project Administration

Maintainers for an area of expertise belong to the associated GitHub team(s) (e.g., `opa-maintainers`, `gatekeeper-maintainers`, etc.) so that GitHub permissions reasonably follow this governance model.

Individuals may be added to that repository's GitHub team but need not be added to the MAINTAINERS.md file. This provision enables new subprojects and contributors to be onboarded without immediately creating new maintainers.