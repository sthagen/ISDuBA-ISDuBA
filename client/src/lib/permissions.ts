export type WorkflowState = string;
export const NEW: WorkflowState = "new";
export const READ: WorkflowState = "read";
export const ASSESSING: WorkflowState = "assessing";
export const REVIEW: WorkflowState = "review";
export const ARCHIVED: WorkflowState = "archived";
export const DELETED: WorkflowState = "deleted";

export const WORKFLOW_STATES = [NEW, READ, ASSESSING, REVIEW, ARCHIVED, DELETED];

export type Role = string;
export const ADMIN: Role = "admin";
export const IMPORTER: Role = "importer";
export const EDITOR: Role = "bearbeiter";
export const REVIEWER: Role = "reviewer";
export const AUDITOR: Role = "auditor";

export type WorkflowStateTransition = {
  from: WorkflowState;
  to: WorkflowState;
  roles: Role[];
};

const WORKFLOW_TRANSITIONS: WorkflowStateTransition[] = [
  {
    from: NEW,
    to: READ,
    roles: [EDITOR]
  },
  { from: READ, to: ASSESSING, roles: [EDITOR] },
  { from: ASSESSING, to: REVIEW, roles: [EDITOR] },
  { from: REVIEW, to: ASSESSING, roles: [REVIEWER] },
  { from: REVIEW, to: ARCHIVED, roles: [REVIEWER] },
  { from: REVIEW, to: DELETED, roles: [REVIEWER] },
  { from: READ, to: DELETED, roles: [EDITOR, REVIEWER] },
  { from: ASSESSING, to: DELETED, roles: [EDITOR, REVIEWER] }
];

function isRoleIncluded(roles: Role[], rolesToCheck: Role[]) {
  for (let i = 0; i < rolesToCheck.length; i++) {
    if (roles.includes(rolesToCheck[i])) {
      return true;
    }
  }
  return false;
}

export function allowedToChangeWorkflow(
  roles: Role[],
  oldState: WorkflowState,
  newState: WorkflowState
) {
  for (let i = 0; i < WORKFLOW_TRANSITIONS.length; i++) {
    const change = WORKFLOW_TRANSITIONS[i];
    if (change.from === oldState && change.to === newState && isRoleIncluded(change.roles, roles)) {
      return true;
    }
  }
  return false;
}

export function getAllowedWorkflowChanges(roles: Role[], currentState: WorkflowState) {
  return WORKFLOW_TRANSITIONS.filter(
    (transition) => isRoleIncluded(transition.roles, roles) && transition.from === currentState
  );
}