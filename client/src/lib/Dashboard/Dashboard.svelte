<!--
 This file is Free Software under the Apache-2.0 License
 without warranty, see README.md and LICENSES/Apache-2.0.txt for details.

 SPDX-License-Identifier: Apache-2.0

 SPDX-FileCopyrightText: 2024 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
 Software-Engineering: 2024 Intevation GmbH <https://intevation.de>
-->

<script lang="ts">
  import { appStore } from "$lib/store";
  import New from "./New.svelte";
  import RecentActivities from "./RecentActivities.svelte";
</script>

<svelte:head>
  <title>Dashboard</title>
</svelte:head>

{#if $appStore.app.isUserLoggedIn}
  <div class="mb-8 mt-8 flex gap-x-10 gap-y-4">
    <New></New>
    <RecentActivities></RecentActivities>
  </div>
{/if}
