---
layout: "powerdns"
page_title: "PowerDNS: Upgrading to v3.0.0"
sidebar_current: "docs-powerdns-guide-upgrading-to-v3"
description: |-
  How to upgrade to v3.0.0, the first release built on the Terraform Plugin Framework.
---

# Upgrading to v3.0.0

Version 3.0.0 is the first release built on the Terraform Plugin Framework. It uses the same provider address as earlier releases, so your existing resources stay under Terraform's control. It needs Terraform 1.0 or later.

## Before you upgrade

1. **Back up the state.** Save a copy you can restore:

   ```bash
   terraform state pull > before-v3.tfstate
   ```

   If your state is in a remote backend, also keep the backend's own version history.

2. **Keep the previous provider version and lock file.** Copy `.terraform.lock.hcl` and note the version constraint you use now, so you can go back:

   ```bash
   cp .terraform.lock.hcl before-v3.terraform.lock.hcl
   ```

3. **Run a plan with the old version** and apply or resolve any pending changes. A clean starting point makes the first v3 plan easier to read.

## Upgrade

1. Change the version constraint:

   ```hcl
   terraform {
     required_providers {
       powerdns = {
         source  = "mmianl/powerdns"
         version = "~> 3.0"
       }
     }
   }
   ```

2. Install the new version and update the lock file:

   ```bash
   terraform init -upgrade
   ```

3. **Read the plan before you apply it.**

   ```bash
   terraform plan
   ```

Existing resources keep their Terraform ownership. They normally need no `terraform import`.

## What to expect on the first apply

The plan can show a few in-place updates. These are expected.

### Spelling is compared by meaning

Some values are compared by meaning, not by spelling. Your state keeps the spelling from your configuration. Examples are:

- a zone `kind` in a different case, such as `MASTER` and `Master`;
- an IPv6 address written in a longer or shorter form, in `masters`, `servers` or `network`;
- a TSIG key name with or without the trailing dot;
- a cryptokey `algorithm` written as a name, a short name such as `ecdsa256`, or its number such as `13`. A number is sent to PowerDNS as the algorithm name.

If you change only the spelling, Terraform shows one in-place update and never replaces the resource. **This update is not always local.** The provider can still send an update request to the server. For example, a change in the case of a zone's `kind` sends a `PUT` request for the zone. The values on the server stay the same, but do not treat these updates as state-only changes.

### SOA serial

The earlier version stored the serial PowerDNS reported, even when `serial = 0` was configured. The new version keeps the `0` you wrote and adds the read-only `server_serial` attribute with the number PowerDNS holds.

- A record with `serial = 0` can show `serial` changing from the stored number to `0` in the first plan. The update sends the serial PowerDNS already holds, so the serial on the server does not go backwards.
- Change references such as `powerdns_record_soa.example.serial` to `powerdns_record_soa.example.server_serial` where you need the real number, for example in outputs or in `triggers_replace`.

See [powerdns_record_soa](/docs/providers/powerdns/r/record_soa.html) for how an omitted, `0` and non-zero serial behave.

### Recursor forward zones with host names

The Recursor replaces a host name in `servers` with its address. The provider keeps your host name in state while the server still reports the address it had for the name. It remembers that address in the provider's private state, which is not shown in plans.

- A change of the address made outside Terraform is shown as a change, and applying restores your configuration.
- The first refresh after the upgrade has nothing remembered yet, so it accepts the address the server reports and remembers it. A change made outside Terraform before that first refresh is not detected.

## Rolling back

**Before the first v3 apply.** `terraform plan` does not change the state that the older provider uses, so you can go back:

1. Restore the old version constraint and the saved lock file (`before-v3.terraform.lock.hcl`).
2. Run `terraform init`.

**After a v3 apply.** The state now has the attributes of v3, such as `server_serial`. An older provider can fail to read it.

1. Restore the old version constraint and lock file.
2. Restore the saved state with `terraform state push before-v3.tfstate`. Terraform refuses to push a state that is older than the current one, and `-force` overrides that, so be careful if other people use the same state.
3. Run `terraform init` and `terraform plan`.

Restoring the state does not undo what v3 already changed on the server, such as a `PUT` for a spelling-only update. Those values are the same as before, so the next plan with the old version should be clean.
