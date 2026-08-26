"use client";

import { useMemo, useState } from "react";
import {
  AlertTriangle,
  ListChecks,
  LockKeyhole,
  Loader2,
  Pencil,
  Plus,
  RefreshCw,
  ShieldCheck,
  Trash2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  AUTHORIZATION_ACTION,
  AUTHORIZATION_RESOURCE,
  POLICY_REASON_LIMITS,
} from "@/constants/authorization";
import { useAuth } from "@/contexts/auth-context";
import { useSession } from "@/contexts/session-context";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { ApiError } from "@/lib/api/error";
import { getErrorMessage, notify } from "@/lib/toast";
import { cn } from "@/lib/utils";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import {
  roleDisplayDescription,
  roleDisplayName,
} from "@/lib/auth/role-presentation";
import { RoleDialog } from "@/modules/admin/dialogs/role-dialog";
import { PermissionMatrix } from "@/modules/admin/permission-matrix";
import { RolePolicyReviewDialog } from "@/modules/admin/role-policy-review-dialog";
import { RoleMetadataReviewDialog } from "@/modules/admin/role-metadata-review-dialog";
import { authService } from "@/services/auth.service";
import { roleService } from "@/services/role.service";
import type {
  AuthorizationCatalog,
  CreateRoleRequest,
  PolicyRule,
  Role,
  RolePolicySnapshot,
  ReviewedRolePolicyCommand,
  ReviewedRoleMetadataCommand,
} from "@/types/admin";

function canonicalRules(rules: PolicyRule[]) {
  return rules
    .map(({ resource, action }) => `${resource}\u0000${action}`)
    .sort()
    .join("\u0001");
}

function requiresRecentAuthentication(error: unknown): boolean {
  return (
    error instanceof ApiError &&
    error.status === 403 &&
    error.params?.reason === "reauthentication_required"
  );
}

export default function AdminRolesPage() {
  const { can, refreshCapabilities } = useAuth();
  const session = useSession();
  const canCreateRole = can(
    AUTHORIZATION_RESOURCE.ROLE,
    AUTHORIZATION_ACTION.CREATE,
  );
  const canUpdateRole = can(
    AUTHORIZATION_RESOURCE.ROLE,
    AUTHORIZATION_ACTION.UPDATE,
  );
  const canManagePolicies = can(
    AUTHORIZATION_RESOURCE.AUTHORIZATION,
    AUTHORIZATION_ACTION.MANAGE,
  );
  const canDeleteRole =
    canManagePolicies &&
    can(AUTHORIZATION_RESOURCE.ROLE, AUTHORIZATION_ACTION.DELETE);
  const [roles, setRoles] = useState<Role[]>([]);
  const [selectedRoleId, setSelectedRoleId] = useState<string | null>(null);
  const [policies, setPolicies] = useState<PolicyRule[]>([]);
  const [originalPolicies, setOriginalPolicies] = useState<PolicyRule[]>([]);
  const [loadedPolicyRoleId, setLoadedPolicyRoleId] = useState<string | null>(
    null,
  );
  const [policyRevision, setPolicyRevision] = useState<number | null>(null);
  const [policyReason, setPolicyReason] = useState("");

  const [isPreviewingPolicies, setIsPreviewingPolicies] = useState(false);
  const [isApplyingPolicies, setIsApplyingPolicies] = useState(false);
  const [isReauthenticatingPolicies, setIsReauthenticatingPolicies] =
    useState(false);
  const [reviewedPolicyCommand, setReviewedPolicyCommand] =
    useState<ReviewedRolePolicyCommand | null>(null);
  const [policyReviewError, setPolicyReviewError] = useState<string | null>(
    null,
  );
  const [policyReauthenticationRequired, setPolicyReauthenticationRequired] =
    useState(false);

  const [roleDialogOpen, setRoleDialogOpen] = useState(false);
  const [editingRole, setEditingRole] = useState<Role | null>(null);
  const [isSavingRole, setIsSavingRole] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Role | null>(null);
  const [isDeletingRole, setIsDeletingRole] = useState(false);
  const [reviewedMetadata, setReviewedMetadata] =
    useState<ReviewedRoleMetadataCommand | null>(null);
  const [metadataBusy, setMetadataBusy] = useState(false);
  const [metadataReauth, setMetadataReauth] = useState(false);
  const [metadataError, setMetadataError] = useState<string | null>(null);

  const rolesResource = useRetryableResource<Role[]>({
    resetKey: "admin-authorization-roles",
    initialData: [],
    load: (signal) => roleService.getAll(signal),
    onSuccess: (nextRoles) => {
      setRoles(nextRoles);
      const nextSelectedRoleId =
        selectedRoleId &&
        nextRoles.some((role) => role.id === selectedRoleId)
          ? selectedRoleId
          : (nextRoles[0]?.id ?? null);
      if (nextSelectedRoleId !== selectedRoleId) {
        resetPolicyProjection();
        setSelectedRoleId(nextSelectedRoleId);
      }
    },
  });
  const catalogResource = useRetryableResource<AuthorizationCatalog | null>({
    resetKey: "admin-authorization-catalog",
    initialData: null,
    load: (signal) => roleService.getCatalog(signal),
  });
  const policyResource = useRetryableResource<RolePolicySnapshot | null>({
    resetKey: selectedRoleId,
    enabled: selectedRoleId !== null,
    initialData: null,
    load: (signal) =>
      selectedRoleId
        ? roleService.getPolicies(selectedRoleId, signal)
        : Promise.resolve(null),
    onSuccess: (snapshot) => {
      if (!snapshot || !selectedRoleId) {
        resetPolicyProjection();
        return;
      }
      setPolicies(snapshot.rules);
      setOriginalPolicies(snapshot.rules);
      setPolicyRevision(snapshot.revision);
      setLoadedPolicyRoleId(selectedRoleId);
    },
  });
  const catalog = catalogResource.data;
  const isLoadingPolicies =
    selectedRoleId !== null &&
    (policyResource.status === "idle" ||
      policyResource.status === "loading" ||
      (policyResource.status === "ready" &&
        loadedPolicyRoleId !== selectedRoleId));
  const policyError =
    policyResource.status === "error"
      ? getErrorMessage(
          policyResource.error,
          ADMIN_TEXT.POLICIES_LOAD_ERROR,
        )
      : null;

  const selectedRole = roles.find((role) => role.id === selectedRoleId) ?? null;
  const selectedRoleDescription = selectedRole
    ? roleDisplayDescription(selectedRole)
    : undefined;
  const isProtectedRole = selectedRole?.is_protected === true;
  const isImmutableRole =
    selectedRole?.is_system === true || selectedRole?.is_protected === true;
  const trimmedPolicyReason = policyReason.trim();
  const isPolicyReasonValid =
    trimmedPolicyReason.length >= POLICY_REASON_LIMITS.MIN_LENGTH &&
    trimmedPolicyReason.length <= POLICY_REASON_LIMITS.MAX_LENGTH;
  const isDirty = useMemo(
    () => canonicalRules(policies) !== canonicalRules(originalPolicies),
    [originalPolicies, policies],
  );
  const isPolicyCommandBusy =
    isPreviewingPolicies ||
    isApplyingPolicies ||
    isReauthenticatingPolicies;
  const canSavePolicies =
    canManagePolicies &&
    Boolean(selectedRoleId) &&
    isDirty &&
    !isProtectedRole &&
    catalogResource.status === "ready" &&
    policyRevision !== null &&
    isPolicyReasonValid &&
    !isPolicyCommandBusy &&
    reviewedPolicyCommand === null &&
    !isLoadingPolicies;

  function resetPolicyProjection() {
    setLoadedPolicyRoleId(null);
    setPolicies([]);
    setOriginalPolicies([]);
    setPolicyRevision(null);
    setPolicyReason("");
  }

  function preparePolicyLoad() {
    resetPolicyProjection();
  }

  function handleSelectRole(roleId: string) {
    if (roleId === selectedRoleId || isPolicyCommandBusy) return;
    if (isDirty && !window.confirm(ADMIN_TEXT.ROLES_UNSAVED_CONFIRM)) return;
    preparePolicyLoad();
    setSelectedRoleId(roleId);
  }

  function handleTogglePolicy(resource: string, action: string, enabled: boolean) {
    if (!selectedRole || isProtectedRole || !canManagePolicies) return;
    setPolicies((current) => {
      const withoutRule = current.filter(
        (rule) => rule.resource !== resource || rule.action !== action,
      );
      if (enabled) return withoutRule;
      return [
        ...withoutRule,
        { role_key: selectedRole.key, resource, action },
      ];
    });
  }

  async function handleSaveRole(input: CreateRoleRequest) {
    if (
      (editingRole &&
        (!canUpdateRole || editingRole.is_system || editingRole.is_protected)) ||
      (!editingRole && !canCreateRole)
    ) {
      return;
    }
    setIsSavingRole(true);
    try {
      if (editingRole) {
        const reviewed = await roleService.previewMetadata(editingRole.id, {
          expected_version: editingRole.version,
          reason: input.reason,
          name: input.name,
          description: input.description ?? "",
        });
        setReviewedMetadata(reviewed);
        setMetadataError(null);
        setMetadataReauth(false);
        setRoleDialogOpen(false);
        return;
      } else {
        const created = await roleService.create(input);
        setRoles((current) =>
          current.some((role) => role.id === created.id)
            ? current.map((role) => (role.id === created.id ? created : role))
            : [...current, created],
        );
        preparePolicyLoad();
        setSelectedRoleId(created.id);
        notify.success(ADMIN_TEXT.ROLES_CREATE_SUCCESS);
        rolesResource.retry();
      }
      setRoleDialogOpen(false);
      await refreshCapabilities();
    } catch (error) {
      if (error instanceof ApiError && error.status === 403) {
        setRoleDialogOpen(false);
        setEditingRole(null);
        notify.error(ADMIN_TEXT.ROLES_SAVE_ERROR);
        rolesResource.retry();
        await refreshCapabilities();
        return;
      }
      if (
        editingRole &&
        error instanceof ApiError &&
        (error.status === 404 || error.status === 409)
      ) {
        setRoleDialogOpen(false);
        setEditingRole(null);
        notify.error(
          error.status === 409
            ? ADMIN_TEXT.ROLES_METADATA_REVIEW.CONFLICT
            : ADMIN_TEXT.ROLES_SAVE_ERROR,
        );
        rolesResource.retry();
        return;
      }
      notify.error(getErrorMessage(error, ADMIN_TEXT.ROLES_SAVE_ERROR));
    } finally {
      setIsSavingRole(false);
    }
  }

  async function handleSavePolicies() {
    if (
      !selectedRoleId ||
      !canManagePolicies ||
      !isDirty ||
      isProtectedRole ||
      policyRevision === null ||
      !isPolicyReasonValid
    ) {
      return;
    }
    const roleID = selectedRoleId;
    setPolicyReviewError(null);
    setPolicyReauthenticationRequired(false);
    setIsPreviewingPolicies(true);
    try {
      const reviewed = await roleService.previewPolicies(roleID, {
        expected_revision: policyRevision,
        reason: trimmedPolicyReason,
        rules: policies,
      });
      setReviewedPolicyCommand(reviewed);
    } catch (error) {
      if (error instanceof ApiError && error.status === 409) {
        notify.error(ADMIN_TEXT.ROLES_POLICY_CONFLICT);
        await refreshCapabilities();
        preparePolicyLoad();
        policyResource.retry();
        return;
      }
      notify.error(
        getErrorMessage(
          error,
          ADMIN_TEXT.ROLES_POLICY_REVIEW.PREVIEW_ERROR,
        ),
      );
    } finally {
      setIsPreviewingPolicies(false);
    }
  }

  function closePolicyReview() {
    if (isApplyingPolicies || isReauthenticatingPolicies) return;
    if (reviewedPolicyCommand) {
      roleService.abandonPolicyCommand(
        reviewedPolicyCommand.role_id,
        reviewedPolicyCommand.command_id,
      );
    }
    setReviewedPolicyCommand(null);
    setPolicyReviewError(null);
    setPolicyReauthenticationRequired(false);
  }

  async function reloadPoliciesAfterConflict(message: string) {
    if (reviewedPolicyCommand) {
      roleService.abandonPolicyCommand(
        reviewedPolicyCommand.role_id,
        reviewedPolicyCommand.command_id,
      );
    }
    setReviewedPolicyCommand(null);
    setPolicyReviewError(null);
    setPolicyReauthenticationRequired(false);
    notify.error(message);
    await refreshCapabilities();
    preparePolicyLoad();
    policyResource.retry();
  }

  async function handleApplyPolicies(currentPassword: string) {
    const reviewed = reviewedPolicyCommand;
    if (!reviewed || isPolicyCommandBusy) return;
    if (Date.parse(reviewed.review.expires_at) <= Date.now()) {
      await reloadPoliciesAfterConflict(
        ADMIN_TEXT.ROLES_POLICY_REVIEW.CONFIRMATION_EXPIRED,
      );
      return;
    }

    setPolicyReviewError(null);
    if (policyReauthenticationRequired) {
      setIsReauthenticatingPolicies(true);
      try {
        await authService.reauthenticateRecoveryContact(currentPassword);
        const refreshed = await session.refresh({ forceRotation: true });
        if (!refreshed) throw new TypeError("session refresh rejected");
        setPolicyReauthenticationRequired(false);
      } catch {
        setPolicyReviewError(ADMIN_TEXT.ROLES_POLICY_REVIEW.REAUTH_ERROR);
        return;
      } finally {
        setIsReauthenticatingPolicies(false);
      }
    }

    setIsApplyingPolicies(true);
    try {
      const receipt = await roleService.applyPolicies(reviewed);
      setPolicies(receipt.snapshot.rules);
      setOriginalPolicies(receipt.snapshot.rules);
      setPolicyRevision(receipt.snapshot.revision);
      setPolicyReason("");
      setReviewedPolicyCommand(null);
      setPolicyReviewError(null);
      setPolicyReauthenticationRequired(false);
      notify.success(ADMIN_TEXT.ROLES_PERMS_SUCCESS);
      await refreshCapabilities();
    } catch (error) {
      if (requiresRecentAuthentication(error)) {
        setPolicyReauthenticationRequired(true);
        setPolicyReviewError(null);
        return;
      }
      if (error instanceof ApiError && error.status === 409) {
        await reloadPoliciesAfterConflict(ADMIN_TEXT.ROLES_POLICY_CONFLICT);
        return;
      }
      if (error instanceof ApiError && !error.retryable) {
        await reloadPoliciesAfterConflict(
          getErrorMessage(
            error,
            ADMIN_TEXT.ROLES_POLICY_REVIEW.APPLY_ERROR,
          ),
        );
        return;
      }
      setPolicyReviewError(
        getErrorMessage(
          error,
          ADMIN_TEXT.ROLES_POLICY_REVIEW.APPLY_ERROR,
        ),
      );
    } finally {
      setIsApplyingPolicies(false);
    }
  }

  async function handleDeleteRole() {
    if (
      !deleteTarget ||
      deleteTarget.is_system ||
      deleteTarget.is_protected ||
      !canDeleteRole
    ) {
      return;
    }
    setIsDeletingRole(true);
    try {
      const reviewed = await roleService.previewMetadata(
        deleteTarget.id,
        {
          expected_version: deleteTarget.version,
          reason: ADMIN_TEXT.ROLES_ARCHIVE_REASON,
        },
        "archive",
      );
      setReviewedMetadata(reviewed);
      setMetadataError(null);
      setMetadataReauth(false);
      setDeleteTarget(null);
    } catch (error) {
      if (error instanceof ApiError && !error.retryable) {
        setDeleteTarget(null);
        notify.error(
          error.status === 409
            ? ADMIN_TEXT.ROLES_METADATA_REVIEW.CONFLICT
            : ADMIN_TEXT.ROLES_DELETE_ERROR,
        );
        rolesResource.retry();
        if (error.status === 403) {
          await refreshCapabilities();
        }
        return;
      }
      notify.error(getErrorMessage(error, ADMIN_TEXT.ROLES_DELETE_ERROR));
    } finally {
      setIsDeletingRole(false);
    }
  }

  function closeMetadataReview() {
    if (metadataBusy) return;
    if (reviewedMetadata) {
      roleService.abandonMetadataCommand(
        reviewedMetadata.role_id,
        reviewedMetadata.review.preview.action,
        reviewedMetadata.command_id,
      );
    }
    setReviewedMetadata(null);
    setMetadataError(null);
    setMetadataReauth(false);
    rolesResource.retry();
  }

  function reloadRolesAfterMetadataConflict(message: string) {
    if (reviewedMetadata) {
      roleService.abandonMetadataCommand(
        reviewedMetadata.role_id,
        reviewedMetadata.review.preview.action,
        reviewedMetadata.command_id,
      );
    }
    setReviewedMetadata(null);
    setMetadataError(null);
    setMetadataReauth(false);
    notify.error(message);
    rolesResource.retry();
  }

  async function applyMetadata(password: string) {
    const reviewed = reviewedMetadata;
    if (!reviewed || metadataBusy) return;
    if (Date.parse(reviewed.review.expires_at) <= Date.now()) {
      reloadRolesAfterMetadataConflict(
        ADMIN_TEXT.ROLES_METADATA_REVIEW.CONFIRMATION_EXPIRED,
      );
      return;
    }

    setMetadataBusy(true);
    setMetadataError(null);
    try {
      if (metadataReauth) {
        try {
          await authService.reauthenticateRecoveryContact(password);
          const refreshed = await session.refresh({ forceRotation: true });
          if (!refreshed) throw new TypeError("session refresh rejected");
          setMetadataReauth(false);
        } catch {
          setMetadataError(ADMIN_TEXT.ROLES_METADATA_REVIEW.REAUTH_ERROR);
          return;
        }
      }

      const receipt = await roleService.applyMetadata(reviewed);
      if (receipt.action === "archive") {
        const nextRoleID =
          roles.find((role) => role.id !== receipt.role.id)?.id ?? null;
        setRoles((current) =>
          current.filter((role) => role.id !== receipt.role.id),
        );
        if (selectedRoleId === receipt.role.id) {
          resetPolicyProjection();
          setSelectedRoleId(nextRoleID);
        }
      } else {
        setRoles((current) =>
          current.map((role) =>
            role.id === receipt.role.id ? receipt.role : role,
          ),
        );
      }
      setEditingRole(null);
      setReviewedMetadata(null);
      setMetadataError(null);
      setMetadataReauth(false);
      notify.success(
        receipt.action === "archive"
          ? ADMIN_TEXT.ROLES_ARCHIVE_SUCCESS
          : ADMIN_TEXT.ROLES_UPDATE_SUCCESS,
      );
      rolesResource.retry();
      await refreshCapabilities();
    } catch (error) {
      if (requiresRecentAuthentication(error)) {
        setMetadataReauth(true);
        setMetadataError(null);
        return;
      }
      if (error instanceof ApiError && error.status === 409) {
        reloadRolesAfterMetadataConflict(
          ADMIN_TEXT.ROLES_METADATA_REVIEW.CONFLICT,
        );
        return;
      }
      if (error instanceof ApiError && !error.retryable) {
        if (error.status === 403) {
          await refreshCapabilities();
        }
        reloadRolesAfterMetadataConflict(
          getErrorMessage(
            error,
            ADMIN_TEXT.ROLES_METADATA_REVIEW.APPLY_ERROR,
          ),
        );
        return;
      }
      setMetadataError(
        getErrorMessage(
          error,
          ADMIN_TEXT.ROLES_METADATA_REVIEW.APPLY_ERROR,
        ),
      );
    } finally {
      setMetadataBusy(false);
    }
  }

  if (rolesResource.status === "loading" && roles.length === 0) {
    return (
      <div className="flex min-h-[50vh] items-center justify-center" role="status">
        <Loader2
          className="size-8 animate-spin text-primary motion-reduce:animate-none"
          aria-hidden="true"
        />
        <span className="sr-only">{TEXT.COMMON.LOADING}</span>
      </div>
    );
  }

  if (rolesResource.status === "error" && roles.length === 0) {
    return (
      <div className="flex min-h-[50vh] items-center justify-center">
        <div className="max-w-lg rounded-2xl border border-destructive/20 bg-destructive/5 p-8 text-center">
          <AlertTriangle
            className="mx-auto size-10 text-destructive"
            aria-hidden="true"
          />
          <h1 className="mt-4 text-lg font-semibold">
            {ADMIN_TEXT.AUTHORIZATION_UNAVAILABLE}
          </h1>
          <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
            {getErrorMessage(
              rolesResource.error,
              ADMIN_TEXT.AUTHORIZATION_LOAD_ERROR,
            )}
          </p>
          <Button
            type="button"
            className="mt-5 cursor-pointer gap-2"
            onClick={rolesResource.retry}
          >
            <RefreshCw className="size-4" aria-hidden="true" />
            {TEXT.COMMON.RETRY}
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <section className="flex flex-col gap-4 rounded-2xl border border-border bg-card/70 p-5 shadow-sm sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-start gap-3">
          <span className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
            <ShieldCheck className="size-5" />
          </span>
          <div>
            <h1 className="text-2xl font-bold tracking-tight">
              {ADMIN_TEXT.ROLES_TITLE}
            </h1>
            <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
              {ADMIN_TEXT.ROLES_SUBTITLE}
            </p>
            {catalog ? (
              <p className="mt-2 text-xs font-medium text-muted-foreground">
                {ADMIN_TEXT.PERMISSIONS.CATALOG_REVISION(catalog.revision)}
              </p>
            ) : null}
          </div>
        </div>
        {canCreateRole ? (
          <Button
            className="min-h-11 cursor-pointer gap-2"
            onClick={() => {
              setEditingRole(null);
              setRoleDialogOpen(true);
            }}
          >
            <Plus className="size-4" />
            {ADMIN_TEXT.ROLES_CREATE}
          </Button>
        ) : null}
      </section>

      {roles.length === 0 ? (
        <div className="rounded-2xl border border-dashed border-border bg-card/40 px-6 py-14 text-center">
          <p className="font-medium">{ADMIN_TEXT.ROLES_EMPTY}</p>
          <p className="mt-1 text-sm text-muted-foreground">
            {ADMIN_TEXT.ROLES_EMPTY_DESCRIPTION}
          </p>
        </div>
      ) : (
        <div className="grid gap-5 xl:grid-cols-[260px_minmax(0,1fr)]">
          <nav
            aria-label={ADMIN_TEXT.ROLES_LIST_LABEL}
            className="flex gap-2 overflow-x-auto rounded-2xl border border-border bg-card/70 p-2 shadow-sm xl:flex-col xl:overflow-visible"
          >
            {roles.map((role) => {
              const isSelected = selectedRoleId === role.id;
              return (
                <button
                  key={role.id}
                  type="button"
                  aria-current={isSelected ? "page" : undefined}
                  onClick={() => handleSelectRole(role.id)}
                  disabled={isPolicyCommandBusy}
                  className={cn(
                    "min-h-11 min-w-44 cursor-pointer rounded-xl border px-3 py-2.5 text-left transition-colors duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring xl:min-w-0",
                    isSelected
                      ? "border-primary/30 bg-primary/10 text-primary"
                      : "border-transparent text-muted-foreground hover:bg-muted hover:text-foreground",
                  )}
                >
                  <span className="block truncate text-sm font-semibold">
                    {roleDisplayName(role)}
                  </span>
                  <code className="mt-0.5 block truncate text-[11px] opacity-75">
                    {role.key}
                  </code>
                </button>
              );
            })}
          </nav>

          {selectedRole ? (
            <section className="min-w-0 space-y-4">
              <div className="flex flex-col gap-3 rounded-2xl border border-border bg-card/70 p-4 shadow-sm sm:flex-row sm:items-center sm:justify-between">
                <div>
                  <div className="flex flex-wrap items-center gap-2">
                    <h2 className="text-lg font-bold">
                      {roleDisplayName(selectedRole)}
                    </h2>
                    {selectedRole.is_system ? (
                      <span className="rounded-full bg-primary/10 px-2 py-0.5 text-[11px] font-semibold text-primary">
                        {ADMIN_TEXT.ROLES_SYSTEM_BADGE}
                      </span>
                    ) : null}
                    {isDirty ? (
                      <span className="rounded-full bg-status-warning/10 px-2 py-0.5 text-[11px] font-semibold text-status-warning">
                        {ADMIN_TEXT.ROLES_UNSAVED_BADGE}
                      </span>
                    ) : null}
                  </div>
                  {selectedRoleDescription ? (
                    <p className="mt-1 text-sm text-muted-foreground">
                      {selectedRoleDescription}
                    </p>
                  ) : null}
                  {policyRevision !== null ? (
                    <p className="mt-1 text-xs font-medium text-muted-foreground">
                      {ADMIN_TEXT.ROLES_POLICY_REVISION(policyRevision)}
                    </p>
                  ) : null}
                </div>
                <div className="flex flex-wrap gap-2">
                  {canUpdateRole && !isImmutableRole ? (
                    <Button
                      variant="outline"
                      className="min-h-11 cursor-pointer gap-2"
                      onClick={() => {
                        setEditingRole(selectedRole);
                        setRoleDialogOpen(true);
                      }}
                      disabled={isPolicyCommandBusy}
                    >
                      <Pencil className="size-4" />
                      {ADMIN_TEXT.ROLES_EDIT}
                    </Button>
                  ) : null}
                  {canManagePolicies ? (
                    <Button
                      className="min-h-11 cursor-pointer gap-2"
                      onClick={handleSavePolicies}
                      disabled={!canSavePolicies}
                    >
                      {isPreviewingPolicies ? (
                        <Loader2 className="size-4 animate-spin motion-reduce:animate-none" aria-hidden="true" />
                      ) : (
                        <ListChecks className="size-4" />
                      )}
                      {isPreviewingPolicies
                        ? ADMIN_TEXT.ROLES_POLICY_REVIEW.PREVIEWING
                        : ADMIN_TEXT.ROLES_SAVE_PERMS}
                    </Button>
                  ) : null}
                  {canDeleteRole && !isImmutableRole ? (
                    <Button
                      variant="destructive"
                      className="min-h-11 cursor-pointer gap-2"
                      onClick={() => setDeleteTarget(selectedRole)}
                      disabled={isPolicyCommandBusy}
                    >
                      <Trash2 className="size-4" />
                      {ADMIN_TEXT.ROLES_ARCHIVE}
                    </Button>
                  ) : null}
                </div>
              </div>

              {isLoadingPolicies ? (
                <div className="flex min-h-72 items-center justify-center rounded-2xl border border-border bg-card/50" role="status">
                  <Loader2 className="size-7 animate-spin text-primary motion-reduce:animate-none" aria-hidden="true" />
                  <span className="sr-only">{ADMIN_TEXT.POLICIES_LOADING}</span>
                </div>
              ) : policyError ? (
                <div className="rounded-2xl border border-destructive/20 bg-destructive/5 p-8 text-center">
                  <AlertTriangle className="mx-auto size-8 text-destructive" />
                  <p className="mt-3 text-sm text-muted-foreground">{policyError}</p>
                  <Button
                    type="button"
                    variant="outline"
                    className="mt-4 cursor-pointer gap-2"
                    onClick={() => {
                      preparePolicyLoad();
                      policyResource.retry();
                    }}
                  >
                    <RefreshCw className="size-4" aria-hidden="true" />
                    {TEXT.COMMON.RETRY}
                  </Button>
                </div>
              ) : (
                <div className="space-y-4">
                  {isProtectedRole || !canManagePolicies ? (
                    <div className="flex gap-3 rounded-2xl border border-primary/20 bg-primary/5 p-4">
                      <LockKeyhole className="mt-0.5 size-5 shrink-0 text-primary" />
                      <div>
                        <h3 className="text-sm font-semibold">
                          {isProtectedRole
                            ? ADMIN_TEXT.ROLES_PROTECTED_TITLE
                            : ADMIN_TEXT.ROLES_READ_ONLY_TITLE}
                        </h3>
                        <p className="mt-1 text-sm leading-relaxed text-muted-foreground">
                          {isProtectedRole
                            ? ADMIN_TEXT.ROLES_PROTECTED_DESCRIPTION
                            : ADMIN_TEXT.ROLES_READ_ONLY_DESCRIPTION}
                        </p>
                      </div>
                    </div>
                  ) : (
                    <div className="rounded-2xl border border-border bg-card/70 p-4 shadow-sm">
                      <div className="flex items-center justify-between gap-3">
                        <Label htmlFor="policy-change-reason">
                          {ADMIN_TEXT.ROLES_POLICY_REASON_LABEL}
                        </Label>
                        <span className="text-xs tabular-nums text-muted-foreground">
                          {ADMIN_TEXT.ROLES_POLICY_REASON_COUNT(
                            policyReason.length,
                            POLICY_REASON_LIMITS.MAX_LENGTH,
                          )}
                        </span>
                      </div>
                      <Textarea
                        id="policy-change-reason"
                        className="mt-2 min-h-24 resize-y"
                        value={policyReason}
                        maxLength={POLICY_REASON_LIMITS.MAX_LENGTH}
                        placeholder={ADMIN_TEXT.ROLES_POLICY_REASON_PLACEHOLDER}
                        aria-describedby="policy-change-reason-hint"
                        aria-invalid={isDirty && !isPolicyReasonValid}
                        disabled={isPolicyCommandBusy || isLoadingPolicies}
                        onChange={(event) => setPolicyReason(event.target.value)}
                      />
                      <p
                        id="policy-change-reason-hint"
                        className={cn(
                          "mt-2 text-xs",
                          isDirty && !isPolicyReasonValid
                            ? "text-destructive"
                            : "text-muted-foreground",
                        )}
                      >
                        {isDirty && !isPolicyReasonValid
                          ? ADMIN_TEXT.ROLES_POLICY_REASON_REQUIRED
                          : ADMIN_TEXT.ROLES_POLICY_REASON_HINT(
                              POLICY_REASON_LIMITS.MIN_LENGTH,
                              POLICY_REASON_LIMITS.MAX_LENGTH,
                            )}
                      </p>
                    </div>
                  )}

                  {catalogResource.status === "loading" ? (
                    <div
                      className="flex min-h-44 items-center justify-center gap-2 rounded-2xl border border-border bg-card/50 text-sm text-muted-foreground"
                      role="status"
                      aria-live="polite"
                    >
                      <Loader2
                        className="size-5 animate-spin text-primary motion-reduce:animate-none"
                        aria-hidden="true"
                      />
                      {ADMIN_TEXT.AUTHORIZATION_CATALOG_LOADING}
                    </div>
                  ) : catalogResource.status === "error" || !catalog ? (
                    <div
                      className="rounded-2xl border border-destructive/20 bg-destructive/5 p-6 text-center"
                      role="alert"
                    >
                      <AlertTriangle
                        className="mx-auto size-7 text-destructive"
                        aria-hidden="true"
                      />
                      <p className="mx-auto mt-3 max-w-xl text-sm leading-6 text-muted-foreground">
                        {ADMIN_TEXT.AUTHORIZATION_CATALOG_ERROR}
                      </p>
                      <Button
                        type="button"
                        variant="outline"
                        className="mt-4 gap-2"
                        onClick={catalogResource.retry}
                      >
                        <RefreshCw className="size-4" aria-hidden="true" />
                        {TEXT.COMMON.RETRY}
                      </Button>
                    </div>
                  ) : (
                    <PermissionMatrix
                      resources={catalog.resources}
                      policies={policies}
                      onToggle={handleTogglePolicy}
                      readOnly={
                        isPolicyCommandBusy ||
                        isProtectedRole ||
                        !canManagePolicies
                      }
                    />
                  )}
                </div>
              )}
            </section>
          ) : null}
        </div>
      )}

      {(editingRole
        ? canUpdateRole && !editingRole.is_system && !editingRole.is_protected
        : canCreateRole) ? (
        <RoleDialog
          key={`${roleDialogOpen}:${editingRole?.id ?? "create"}`}
          open={roleDialogOpen}
          editing={editingRole}
          onSave={handleSaveRole}
          onClose={() => {
            setRoleDialogOpen(false);
            rolesResource.retry();
          }}
          isSaving={isSavingRole}
        />
      ) : null}
      {canDeleteRole ? (
        <ConfirmDialog
          open={deleteTarget !== null}
          onOpenChange={(open) => {
            if (!open && !isDeletingRole) setDeleteTarget(null);
          }}
          onConfirm={handleDeleteRole}
          loading={isDeletingRole}
          title={ADMIN_TEXT.ROLES_ARCHIVE_TITLE}
          description={ADMIN_TEXT.ROLES_ARCHIVE_DESCRIPTION(
            deleteTarget ? roleDisplayName(deleteTarget) : "",
          )}
        />
      ) : null}
      <RolePolicyReviewDialog
        open={reviewedPolicyCommand !== null}
        role={selectedRole}
        reviewed={reviewedPolicyCommand}
        applying={isApplyingPolicies}
        reauthenticating={isReauthenticatingPolicies}
        requiresReauthentication={policyReauthenticationRequired}
        operationError={policyReviewError}
        onOpenChange={(open) => {
          if (!open) closePolicyReview();
        }}
        onConfirm={(password) => void handleApplyPolicies(password)}
      />
      <RoleMetadataReviewDialog
        reviewed={reviewedMetadata}
        busy={metadataBusy}
        requiresReauthentication={metadataReauth}
        error={metadataError}
        onClose={closeMetadataReview}
        onConfirm={(password) => void applyMetadata(password)}
      />
    </div>
  );
}
