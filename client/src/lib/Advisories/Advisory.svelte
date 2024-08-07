<!--
 This file is Free Software under the Apache-2.0 License
 without warranty, see README.md and LICENSES/Apache-2.0.txt for details.

 SPDX-License-Identifier: Apache-2.0

 SPDX-FileCopyrightText: 2024 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
 Software-Engineering: 2024 Intevation GmbH <https://intevation.de>
-->
<script lang="ts">
  import { Label, Badge } from "flowbite-svelte";
  import { onDestroy } from "svelte";
  import { appStore } from "$lib/store";
  import Version from "$lib/Advisories/Version.svelte";
  import Webview from "$lib/Advisories/CSAFWebview/Webview.svelte";
  import { convertToDocModel } from "$lib/Advisories/CSAFWebview/docmodel/docmodel";
  import SsvcCalculator from "$lib/Advisories/SSVC/SSVCCalculator.svelte";
  import { convertVectorToLabel } from "$lib/Advisories/SSVC/SSVCCalculator";
  import Diff from "$lib/Diff/Diff.svelte";
  import { ARCHIVED, ASSESSING, DELETE, NEW, READ, REVIEW } from "$lib/workflow";
  import { canSetStateRead } from "$lib/permissions";
  import CommentTextArea from "./Comments/CommentTextArea.svelte";
  import { request } from "$lib/utils";
  import ErrorMessage from "$lib/Errors/ErrorMessage.svelte";
  import { getErrorMessage } from "$lib/Errors/error";
  import WorkflowStates from "./WorkflowStates.svelte";
  import History from "./History.svelte/History.svelte";
  import Tlp from "./TLP.svelte";
  export let params: any = null;

  let document: any = {};
  let ssvc: any;
  let comment: string = "";
  let loadCommentsError = "";
  let loadEventsError = "";
  let loadAdvisoryVersionsError = "";
  let loadDocumentError = "";
  let loadFourCVEsError = "";
  let createCommentError = "";
  let loadDocumentSSVCError = "";
  let stateError = "";
  let advisoryVersions: any[] = [];
  let advisoryVersionByDocumentID: any;
  let advisoryState = "";
  let historyEntries: any = [];
  let isCommentingAllowed: boolean;
  let isSSVCediting = false;
  $: if ([NEW, READ, ASSESSING, REVIEW, ARCHIVED].includes(advisoryState)) {
    if (appStore.isReviewer() && [NEW, READ, ARCHIVED].includes(advisoryState)) {
      isCommentingAllowed = false;
    } else {
      isCommentingAllowed = appStore.isEditor() || appStore.isReviewer();
    }
  } else {
    isCommentingAllowed = false;
  }
  let isCalculatingAllowed: boolean;
  $: if ([NEW, READ, ASSESSING].includes(advisoryState)) {
    isCalculatingAllowed = appStore.isEditor() || appStore.isReviewer();
  } else {
    isCalculatingAllowed = false;
  }

  const setAsReadTimeout: number[] = [];
  let isDiffOpen = false;
  let commentFocus = false;

  const loadAdvisoryVersions = async () => {
    const response = await request(
      `/api/documents?&columns=id version tracking_id&query=$tracking_id ${params.trackingID} = $publisher "${params.publisherNamespace}" = and`,
      "GET"
    );
    if (response.ok) {
      const result = await response.content;
      advisoryVersions = result.documents.map((doc: any) => {
        return { id: doc.id, version: doc.version, tracking_id: doc.tracking_id };
      });
      advisoryVersionByDocumentID = advisoryVersions.reduce((acc: any, version: any) => {
        acc[version.id] = version.version;
        return acc;
      }, {});
    } else if (response.error) {
      loadAdvisoryVersionsError = `Could not load versions. ${getErrorMessage(response.error)}`;
    }
  };

  const loadDocument = async () => {
    const response = await request(`/api/documents/${params.id}`, "GET");
    if (response.ok) {
      const result = await response.content;
      ({ document } = result);
      const docModel = convertToDocModel(result);
      appStore.setDocument(docModel);
    } else if (response.error) {
      loadDocumentError = `Could not load document. ${getErrorMessage(response.error)}`;
    }
  };

  const loadDocumentSSVC = async () => {
    const response = await request(
      `/api/documents?columns=ssvc&query=$tracking_id ${params.trackingID} = $publisher "${params.publisherNamespace}" = and`,
      "GET"
    );
    if (response.ok) {
      const result = await response.content;
      if (result.documents[0].ssvc) {
        ssvc = convertVectorToLabel(result.documents[0].ssvc);
      }
    } else if (response.error) {
      loadDocumentSSVCError = `Could not load SSVC. ${getErrorMessage(response.error)}`;
    }
  };

  const loadEvents = async () => {
    const response = await request(
      `/api/events/${params.publisherNamespace}/${params.trackingID}`,
      "GET"
    );
    if (response.ok) {
      return await response.content;
    } else if (response.error) {
      loadEventsError = `Could not load events. ${getErrorMessage(response.error)}`;
      return [];
    }
  };

  const loadComments = async () => {
    const response = await request(
      `/api/comments/${params.publisherNamespace}/${params.trackingID}`,
      "GET"
    );
    if (response.ok) {
      let comments = await response.content;
      for (let i = 0; i < comments.length; i++) {
        comments[i].documentVersion = advisoryVersionByDocumentID[comments[i].document_id];
      }
      return comments;
    } else if (response.error) {
      loadEventsError = `Could not comments. ${getErrorMessage(response.error)}`;
      return [];
    }
  };

  const buildHistory = async () => {
    const comments = await loadComments();
    let events = await loadEvents();
    const commentsByTime = comments.reduce((o: any, n: any) => {
      o[`${n.time}:${n.commentator}`] = {
        message: n.message,
        id: n.id,
        documentVersion: n.documentVersion
      };
      return o;
    }, {});
    const commentsEdited = events
      .filter((e: any) => {
        return e.event_type === "change_comment";
      })
      .map((e: any) => {
        return {
          id: e.comment_id,
          time: e.time
        };
      })
      .reduce((o: any, n: any) => {
        if (!o[n.id]) o[n.id] = [];
        o[n.id].push(n.time);
        return o;
      }, {});
    events.map((e: any) => {
      if (e.event_type === "add_comment") {
        const comment = commentsByTime[`${e.time}:${e.actor}`];
        e["message"] = comment.message;
        e["comment_id"] = comment.id;
        e["documentVersion"] = comment.documentVersion;
        if (commentsEdited[comment.id]) {
          e["times"] = commentsEdited[comment.id];
        }
      }
      return e;
    });
    historyEntries = events;
  };

  async function createComment() {
    await allowEditing();
    const formData = new FormData();
    formData.append("message", comment);
    const response = await request(`/api/comments/${params.id}`, "POST", formData);
    if (response.ok) {
      comment = "";
      await loadAdvisoryState();
      await buildHistory();
    } else if (response.error) {
      createCommentError = `Could not create comment. ${getErrorMessage(response.error)}`;
    }
  }

  async function sendForReview() {
    if (comment.length !== 0) {
      await createComment();
    }
    await updateState(REVIEW);
  }

  async function sendForAssessing() {
    if (comment.length !== 0) {
      await createComment();
    }
    await updateState(ASSESSING);
  }

  async function updateState(newState: string) {
    // Cancel automatic state transitions
    setAsReadTimeout.forEach((id: number) => {
      clearTimeout(id);
    });

    const response = await request(
      `/api/status/${params.publisherNamespace}/${params.trackingID}/${newState}`,
      "PUT"
    );
    if (response.ok) {
      advisoryState = newState;
      await buildHistory();
    } else if (response.error) {
      stateError = `Could not change state. ${getErrorMessage(response.error)}`;
    }
  }

  const loadAdvisoryState = async () => {
    const response = await request(
      `/api/documents?advisories=true&columns=state&query=$tracking_id ${params.trackingID} = $publisher "${params.publisherNamespace}" = and`,
      "GET"
    );
    if (response.ok) {
      const result = response.content;
      advisoryState = result.documents[0].state;
      return result.documents[0].state;
    } else if (response.error) {
      stateError = `Couldn't load state. ${getErrorMessage(response.error)}`;
    }
  };

  const loadFourCVEs = async () => {
    const response = await request(
      `/api/documents?advisories=true&columns=four_cves&query=$id ${params.id} integer =`,
      "GET"
    );
    if (response.ok) {
      const content = await response.content;
      let four_cves = content?.documents[0]?.four_cves;
      appStore.setFourCVEs(four_cves);
    } else if (response.error) {
      loadFourCVEsError = `Couldn't load CVEs. ${getErrorMessage(response.error)}`;
    }
  };

  const loadData = async () => {
    await loadFourCVEs();
    await loadDocumentSSVC();
    await loadDocument();
    await loadAdvisoryVersions();
    await buildHistory();
    if (appStore.isEditor() || appStore.isReviewer() || appStore.isAuditor()) {
      await buildHistory();
    }
    await loadAdvisoryState();
    // Only set state to 'read' if editor opens the current version.
    if (
      advisoryState === NEW &&
      canSetStateRead(advisoryState) &&
      (advisoryVersions.length === 1 || advisoryVersions[0].version === document.tracking?.version)
    ) {
      const id: any = setTimeout(async () => {
        if (advisoryState === "new" && canSetStateRead(advisoryState)) {
          await updateState(READ);
        }
      }, 20000);
      setAsReadTimeout.push(id);
    }
  };

  async function loadMetaData() {
    await loadAdvisoryState();
    await loadDocumentSSVC();
    await buildHistory();
  }

  async function allowEditing() {
    if (advisoryState === NEW && canSetStateRead(advisoryState)) {
      await updateState(READ);
    }
  }

  onDestroy(() => {
    setAsReadTimeout.forEach((id: number) => {
      clearTimeout(id);
    });
  });

  $: if (params) {
    loadData();
  }
  $: ssvcStyle = ssvc ? `color: white; background-color: ${ssvc.color};` : "";
</script>

<svelte:head>
  <title>{params.trackingID}</title>
</svelte:head>

<div class="flex h-screen max-h-full flex-wrap justify-between gap-x-4 gap-y-8 xl:flex-nowrap">
  <div class="flex max-h-full w-full grow flex-col gap-y-2 px-2">
    <div class="flex flex-col">
      <div class="flex gap-2">
        <Label class="text-lg">
          <span class="mr-2">{params.trackingID}</span>
          <Tlp tlp={$appStore.webview.doc?.tlp.label}></Tlp>
        </Label>
      </div>
      <div class="flex flex-row flex-wrap items-end justify-start gap-y-2 md:justify-between">
        <Label class="text-gray-600">{params.publisherNamespace}</Label>
        <div class="right-6 mt-4 flex h-fit flex-row gap-2 min-[1080px]:absolute">
          <WorkflowStates {advisoryState} updateStateFn={updateState}></WorkflowStates>
        </div>
      </div>
      <div class="mb-4 mt-2" />
    </div>
    <ErrorMessage message={loadAdvisoryVersionsError}></ErrorMessage>
    <ErrorMessage message={loadDocumentSSVCError}></ErrorMessage>
    <ErrorMessage message={stateError}></ErrorMessage>
    <ErrorMessage message={loadDocumentError}></ErrorMessage>
    <ErrorMessage message={loadFourCVEsError}></ErrorMessage>
    <div class="flex flex-row max-[800px]:flex-wrap-reverse">
      <div class="mr-12 flex w-2/3 flex-col">
        <div class="flex flex-row">
          {#if advisoryVersions.length > 0}
            <Version
              publisherNamespace={params.publisherNamespace}
              trackingID={params.trackingID}
              {advisoryVersions}
              selectedDocumentVersion={document.tracking?.version}
              on:selectedDiffDocuments={() => (isDiffOpen = true)}
              on:disableDiff={() => (isDiffOpen = false)}
            ></Version>
          {/if}
        </div>
        <div class="flex flex-col min-[800px]:mr-56 min-[1080px]:mr-32">
          {#if isDiffOpen}
            <Diff showTitle={false}></Diff>
          {:else}
            <Webview></Webview>
          {/if}
        </div>
      </div>
      <div
        class="right-3 mr-3 flex w-[29rem] flex-col bg-white min-[800px]:ml-auto min-[1080px]:absolute"
      >
        <div class={isSSVCediting || commentFocus ? " w-full p-3 shadow-md" : "w-full p-3"}>
          <div class="mb-4 flex flex-row items-center">
            {#if ssvc}
              {#if !isSSVCediting}
                <Badge class="h-6 w-fit" title={ssvc.vector} style={ssvcStyle}>{ssvc.label}</Badge>
              {/if}
            {/if}
            {#if advisoryState !== ARCHIVED && advisoryState !== DELETE}
              <SsvcCalculator
                bind:isEditing={isSSVCediting}
                vectorInput={ssvc?.vector}
                disabled={!isCalculatingAllowed}
                documentID={params.id}
                on:updateSSVC={loadMetaData}
                {allowEditing}
              ></SsvcCalculator>
            {/if}
          </div>
          {#if isCommentingAllowed && !isSSVCediting}
            <div class="mt-6">
              <Label class="mb-2" for="comment-textarea"
                >{advisoryState === ARCHIVED ? "Reactivate with comment" : "New Comment"}</Label
              >
              <CommentTextArea
                on:focus={() => {
                  commentFocus = true;
                }}
                on:blur={() => {
                  commentFocus = false;
                }}
                on:input={() => (createCommentError = "")}
                on:saveComment={createComment}
                on:saveForReview={sendForReview}
                on:saveForAssessing={sendForAssessing}
                bind:value={comment}
                errorMessage={createCommentError}
                buttonText="Send"
                state={advisoryState}
              ></CommentTextArea>
            </div>
          {/if}
        </div>
        <ErrorMessage message={loadDocumentSSVCError}></ErrorMessage>
        <div class="">
          {#if appStore.isEditor() || appStore.isReviewer() || appStore.isAuditor()}
            <div class="mt-6">
              <History
                state={advisoryState}
                on:commentUpdate={() => {
                  buildHistory();
                }}
                entries={historyEntries}
              ></History>
            </div>
            <ErrorMessage message={loadEventsError}></ErrorMessage>
            <ErrorMessage message={loadCommentsError}></ErrorMessage>
          {/if}
        </div>
      </div>
    </div>
  </div>
</div>
