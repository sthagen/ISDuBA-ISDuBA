// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSES/Apache-2.0.txt for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2024 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
//  Software-Engineering: 2024 Intevation GmbH <https://intevation.de>

import decisionTree from "./CISA-Coordinator";

export type SSVCDecisionChildCombinationItem = {
  child_key?: string;
  child_label: string;
  child_option_keys?: string[];
  child_option_labels: string[];
};

export type SSVCOption = {
  label: string;
  key: string;
  description: string;
  child_combinations?: SSVCDecisionChildCombinationItem[][];
  color?: string;
};

export type SSVCDecisionChild = {
  label: string;
};

export type SSVCDecision = {
  label: string;
  key: string;
  decision_type: string;
  children?: SSVCDecisionChild[];
  options: SSVCOption[];
};

export type SSVCDecisionTree = {
  decision_points: SSVCDecision[];
  decisions_table: any[];
  lang: string;
  title: string;
  version: string;
};

export interface SSVCObject {
  vector: string;
  color: string;
  label: string;
}

export function parseDecisionTree() {
  const json: SSVCDecisionTree = decisionTree;
  const addedPoints: string[] = [];
  const decisionPoints: SSVCDecision[] = json.decision_points;
  const decisionsTable = json.decisions_table;
  let mainDecisions: SSVCDecision[] = [];
  for (let i = decisionPoints.length - 1; i >= 0; i--) {
    const decision: SSVCDecision = decisionPoints[i];
    if (!addedPoints.includes(decision.label)) {
      mainDecisions.push(decision);
      if (decision.decision_type === "complex") {
        for (const child of decision.children || []) {
          addedPoints.push(child.label);
        }
      } else {
        addedPoints.push(decision.label);
      }
    }
  }
  mainDecisions = mainDecisions.reverse();
  const steps = mainDecisions.map((element) => element.label);
  return {
    decisionPoints: decisionPoints,
    decisionsTable: decisionsTable,
    mainDecisions: mainDecisions,
    steps: steps
  };
}

export function getDecision(
  decisionPoints: SSVCDecision[],
  label: string
): SSVCDecision | undefined {
  return decisionPoints.find((element) => element.label === label);
}

export function getOptionWithKey(decision: SSVCDecision, key: string): SSVCOption | undefined {
  return decision.options.find((element: SSVCOption) => element.key === key);
}

export function createIsoTimeStringForSSVC() {
  const iso = new Date().toISOString();
  return `${iso.split(".")[0]}Z`;
}

export function convertVectorToSSVCObject(vector: string): SSVCObject {
  const { mainDecisions, decisionPoints } = parseDecisionTree();
  console.log(vector);
  const keyPairs = vector.split("/").slice(1, -2);
  let selectedOption: SSVCOption | undefined;
  const keyOfSelectedOption = keyPairs[keyPairs.length - 1].split(":")[1];
  if (mainDecisions.length === keyPairs.length) {
    selectedOption = getOptionWithKey(mainDecisions[mainDecisions.length - 1], keyOfSelectedOption);
  } else if (decisionPoints.length === keyPairs.length) {
    selectedOption = getOptionWithKey(
      decisionPoints[decisionPoints.length - 1],
      keyOfSelectedOption
    );
  }
  return {
    vector: vector,
    label: selectedOption?.label ?? "",
    color: selectedOption?.color ?? ""
  };
}
