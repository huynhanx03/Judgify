"use client";

/**
 * Admin user management — invite, resend, authorize, and manage lifecycle.
 */

import { useMemo, useState } from "react";
import {
  CircleAlert,
  Loader2,
  PauseCircle,
  PlayCircle,
  Plus,
  RefreshCw,
  Search,
  Trash2,
  UserCog,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { AdminDataTable, type AdminColumn } from "@/modules/admin/admin-data-table";
import { EditRoleDialog } from "@/modules/admin/dialogs/edit-role-dialog";
import {
  InviteUserDialog,
  type InviteUserReviewResult,
} from "@/modules/admin/dialogs/create-user-dialog";
import { UserRoleReviewDialog } from "@/modules/admin/user-role-review-dialog";
import { UserLifecycleDialog } from "@/modules/admin/user-lifecycle-dialog";
import { UserInvitationResendDialog } from "@/modules/admin/user-invitation-resend-dialog";
import {
  isRecentAuthenticationRequired,
  userService,
} from "@/services/user.service";
import { roleService } from "@/services/role.service";
import { authService } from "@/services/auth.service";
import { roleDisplayName } from "@/lib/auth/role-presentation";
import {
  invalidProfileValueDefinitionID,
  isProfileSchemaConflict,
} from "@/lib/auth/onboarding-profile-schema";
import { ApiError } from "@/lib/api/error";
import { notify, getErrorMessage } from "@/lib/toast";
import { TEXT } from "@/constants/text";
import { ADMIN_TEXT } from "@/constants/admin-text";
import { IDENTITY_INPUT_LIMITS } from "@/constants/identity";
import { AUTHORIZATION_ACTION, AUTHORIZATION_RESOURCE } from "@/constants/authorization";
import { useAuth } from "@/contexts/auth-context";
import { useSession } from "@/contexts/session-context";
import { useRetryableResource } from "@/hooks/use-retryable-resource";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import type {
  AdminUser,
  ReviewedUserInvitationCommand,
  ReviewedUserInvitationResendCommand,
  ReviewedUserLifecycleCommand,
  ReviewedUserRoleCommand,
  Role,
  UserInvitationCommandInput,
  UserLifecycleAction,
  UserLifecycleCommandInput,
  UserRoleCommandInput,
} from "@/types/admin";
import type { Paginated, PaginationMeta, QueryOptions } from "@/types/api";

const ROLE_COLORS: Record<string, string> = {
  admin: "border-danger/20 bg-danger/10 text-danger",
  teacher: "border-info/20 bg-info/10 text-info",
  student: "border-success/20 bg-success/10 text-success",
};

function currentRevisionFromConflict(error: ApiError): number | null {
  const readRevision = (value: unknown): number | null => {
    if (!value || typeof value !== "object") return null;
    const record = value as Record<string, unknown>;
    if (
      typeof record.current_revision === "number" &&
      Number.isInteger(record.current_revision) &&
      record.current_revision > 0
    ) {
      return record.current_revision;
    }
    return readRevision(record.details);
  };

  return readRevision(error.params);
}

function currentVersionFromConflict(error: ApiError): number | null {
  const readVersion = (value: unknown): number | null => {
    if (!value || typeof value !== "object") return null;
    const record = value as Record<string, unknown>;
    if (
      typeof record.current_version === "number" &&
      Number.isSafeInteger(record.current_version) &&
      record.current_version > 0
    ) {
      return record.current_version;
    }
    return readVersion(record.details);
  };

  return readVersion(error.params);
}

function isFinalSuperadminConflict(error: ApiError): boolean {
  return error.params?.reason === "final_superadmin_required";
}

export default function AdminUsersPage() {
  const {
    authorizationRevision,
    can,
    refreshCapabilities,
    user: currentUser,
  } = useAuth();
  const session = useSession();
  const canReadUsers = can(
    AUTHORIZATION_RESOURCE.USER,
    AUTHORIZATION_ACTION.READ,
  );
  const canManageUsers = can(
    AUTHORIZATION_RESOURCE.AUTHORIZATION,
    AUTHORIZATION_ACTION.MANAGE,
  );
  const canCreateUser =
    canManageUsers &&
    can(AUTHORIZATION_RESOURCE.USER, AUTHORIZATION_ACTION.CREATE);
  const canUpdateUser =
    canManageUsers &&
    can(AUTHORIZATION_RESOURCE.USER, AUTHORIZATION_ACTION.UPDATE);
  const canDeleteUser =
    canManageUsers &&
    can(AUTHORIZATION_RESOURCE.USER, AUTHORIZATION_ACTION.DELETE);
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [pagination, setPagination] = useState<PaginationMeta>();
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(
    search.trim(),
    IDENTITY_INPUT_LIMITS.SEARCH_DEBOUNCE_MS,
  );

  // User lifecycle state
  const [lifecycleTarget, setLifecycleTarget] = useState<AdminUser | null>(null);
  const [lifecycleAction, setLifecycleAction] =
    useState<UserLifecycleAction | null>(null);
  const [reviewedLifecycleCommand, setReviewedLifecycleCommand] =
    useState<ReviewedUserLifecycleCommand | null>(null);
  const [isReviewingLifecycle, setIsReviewingLifecycle] = useState(false);
  const [isApplyingLifecycle, setIsApplyingLifecycle] = useState(false);
  const [isReauthenticatingLifecycle, setIsReauthenticatingLifecycle] =
    useState(false);
  const [lifecycleReauthenticationRequired, setLifecycleReauthenticationRequired] =
    useState(false);
  const [lifecycleError, setLifecycleError] = useState<string | null>(null);

  // Edit role state
  const [editTarget, setEditTarget] = useState<AdminUser | null>(null);
  const [roleDraft, setRoleDraft] = useState<UserRoleCommandInput | null>(null);
  const [reviewedRoleCommand, setReviewedRoleCommand] =
    useState<ReviewedUserRoleCommand | null>(null);
  const [isReviewingRole, setIsReviewingRole] = useState(false);
  const [isApplyingRole, setIsApplyingRole] = useState(false);
  const [isReauthenticatingRole, setIsReauthenticatingRole] = useState(false);
  const [roleReauthenticationRequired, setRoleReauthenticationRequired] =
    useState(false);
  const [roleReviewError, setRoleReviewError] = useState<string | null>(null);

  // Reviewed administrative invitation state
  const [createOpen, setCreateOpen] = useState(false);
  const [reviewedInvitation, setReviewedInvitation] =
    useState<ReviewedUserInvitationCommand | null>(null);
  const [isReviewingInvitation, setIsReviewingInvitation] = useState(false);
  const [isApplyingInvitation, setIsApplyingInvitation] = useState(false);
  const [isReauthenticatingInvitation, setIsReauthenticatingInvitation] =
    useState(false);
  const [invitationReauthenticationRequired, setInvitationReauthenticationRequired] =
    useState(false);
  const [invitationError, setInvitationError] = useState<string | null>(null);

  // Reviewed invitation resend state. The contact version remains server-owned.
  const [invitationResendTarget, setInvitationResendTarget] =
    useState<AdminUser | null>(null);
  const [reviewedInvitationResend, setReviewedInvitationResend] =
    useState<ReviewedUserInvitationResendCommand | null>(null);
  const [isReviewingInvitationResend, setIsReviewingInvitationResend] =
    useState(false);
  const [isApplyingInvitationResend, setIsApplyingInvitationResend] =
    useState(false);
  const [isReauthenticatingInvitationResend, setIsReauthenticatingInvitationResend] =
    useState(false);
  const [invitationResendReauthenticationRequired, setInvitationResendReauthenticationRequired] =
    useState(false);
  const [invitationResendError, setInvitationResendError] =
    useState<string | null>(null);

  const usersQuery = useMemo<QueryOptions>(() => {
    const normalizedSearch = debouncedSearch.trim();
    return {
      pagination: {
        page,
        page_size: IDENTITY_INPUT_LIMITS.ADMIN_USERS_PAGE_SIZE,
      },
      filters: normalizedSearch
        ? [{ key: "username", value: normalizedSearch, type: "search" }]
        : [],
    };
  }, [debouncedSearch, page]);

  const usersResource = useRetryableResource<Paginated<AdminUser> | null>({
    resetKey: `${authorizationRevision}:${page}:${debouncedSearch.trim()}`,
    enabled: canReadUsers,
    initialData: null,
    keepPreviousData: true,
    load: (signal) => userService.find(usersQuery, signal),
    onSuccess: (response) => {
      if (!response) return;
      setUsers(response.records);
      setPagination(response.pagination);
    },
  });

  const rolesResource = useRetryableResource<Role[]>({
    resetKey: `${authorizationRevision}:${canManageUsers}`,
    enabled: canManageUsers,
    initialData: [],
    load: (signal) => roleService.getAll(signal),
  });
  const roles = rolesResource.data;

  function openLifecycleDialog(
    target: AdminUser,
    action: UserLifecycleAction,
  ): void {
    setLifecycleTarget(target);
    setLifecycleAction(action);
    setReviewedLifecycleCommand(null);
    setLifecycleError(null);
    setLifecycleReauthenticationRequired(false);
  }

  function closeLifecycleDialog(): void {
    if (reviewedLifecycleCommand) {
      userService.abandonLifecycleCommand(reviewedLifecycleCommand);
    }
    setLifecycleTarget(null);
    setLifecycleAction(null);
    setReviewedLifecycleCommand(null);
    setLifecycleError(null);
    setLifecycleReauthenticationRequired(false);
  }

  function returnToLifecycleEditor(): void {
    if (reviewedLifecycleCommand) {
      userService.abandonLifecycleCommand(reviewedLifecycleCommand);
    }
    setReviewedLifecycleCommand(null);
    setLifecycleError(null);
    setLifecycleReauthenticationRequired(false);
  }

  async function refreshUserAfterLifecycleConflict(
    userId: string,
    currentVersion: number | null,
  ): Promise<boolean> {
    try {
      const response = await userService.find(usersQuery);
      const refreshedUser = response.records.find((user) => user.id === userId);
      setUsers(response.records);
      setPagination(response.pagination);
      if (refreshedUser) {
        setLifecycleTarget(refreshedUser);
        return true;
      }
    } catch {
      // Preserve the reviewed intent and expose a safe retry message below.
    }

    if (currentVersion !== null) {
      setLifecycleTarget((current) =>
        current?.id === userId
          ? { ...current, version: currentVersion }
          : current,
      );
    }
    usersResource.retry();
    return false;
  }

  async function handleReviewLifecycle(
    input: UserLifecycleCommandInput,
  ): Promise<void> {
    const target = lifecycleTarget;
    const action = lifecycleAction;
    const permitted =
      action === "delete" ? canDeleteUser : canUpdateUser;
    if (!target || !action || !permitted) return;

    setIsReviewingLifecycle(true);
    setLifecycleError(null);
    try {
      const reviewed = await userService.previewLifecycle(target, action, input);
      setReviewedLifecycleCommand(reviewed);
    } catch (error) {
      if (error instanceof ApiError && error.status === 409) {
        const finalSuperadminRequired = isFinalSuperadminConflict(error);
        const reloaded = await refreshUserAfterLifecycleConflict(
          target.id,
          currentVersionFromConflict(error),
        );
        setLifecycleError(
          finalSuperadminRequired
            ? ADMIN_TEXT.USERS_FINAL_SUPERADMIN_REQUIRED
            : reloaded
            ? ADMIN_TEXT.USERS_LIFECYCLE.CONFLICT_RELOADED
            : ADMIN_TEXT.USERS_LIFECYCLE.CONFLICT_RETRY,
        );
      } else {
        setLifecycleError(
          getErrorMessage(error, ADMIN_TEXT.USERS_LIFECYCLE.PREVIEW_ERROR),
        );
      }
    } finally {
      setIsReviewingLifecycle(false);
    }
  }

  async function handleApplyLifecycle(currentPassword: string): Promise<void> {
    const reviewed = reviewedLifecycleCommand;
    if (!reviewed) return;
    const permitted =
      reviewed.action === "delete" ? canDeleteUser : canUpdateUser;
    if (!permitted) return;

    if (Date.parse(reviewed.review.expires_at) <= Date.now()) {
      userService.abandonLifecycleCommand(reviewed);
      setReviewedLifecycleCommand(null);
      setLifecycleReauthenticationRequired(false);
      setLifecycleError(ADMIN_TEXT.USERS_LIFECYCLE.EXPIRED);
      return;
    }

    setLifecycleError(null);
    if (lifecycleReauthenticationRequired) {
      setIsReauthenticatingLifecycle(true);
      try {
        await authService.reauthenticateRecoveryContact(currentPassword);
        const refreshed = await session.refresh({ forceRotation: true });
        if (!refreshed) {
          throw new TypeError("session refresh did not yield a session");
        }
        setLifecycleReauthenticationRequired(false);
      } catch (error) {
        setLifecycleError(
          getErrorMessage(error, ADMIN_TEXT.USERS_LIFECYCLE.REAUTH_ERROR),
        );
        return;
      } finally {
        setIsReauthenticatingLifecycle(false);
      }
    }

    setIsApplyingLifecycle(true);
    try {
      const receipt = await userService.applyLifecycle(reviewed);
      if (receipt.action === "delete") {
        setUsers((current) =>
          current.filter((user) => user.id !== receipt.user.id),
        );
        if (users.length === 1 && page > 1) {
          setPage((current) => current - 1);
        } else {
          usersResource.retry();
        }
      } else {
        setUsers((current) =>
          current.map((user) =>
            user.id === receipt.user.id
              ? {
                  ...user,
                  status: receipt.user.status,
                  version: receipt.user.version,
                  suspended_at: receipt.user.suspended_at,
                }
              : user,
          ),
        );
      }
      setLifecycleTarget(null);
      setLifecycleAction(null);
      setReviewedLifecycleCommand(null);
      setLifecycleError(null);
      setLifecycleReauthenticationRequired(false);
      notify.success(ADMIN_TEXT.USERS_LIFECYCLE.SUCCESS(receipt.action));
    } catch (error) {
      if (isRecentAuthenticationRequired(error)) {
        setLifecycleReauthenticationRequired(true);
        setLifecycleError(ADMIN_TEXT.USERS_LIFECYCLE.REAUTH_REQUIRED);
        return;
      }
      if (error instanceof ApiError && error.status === 409) {
        const finalSuperadminRequired = isFinalSuperadminConflict(error);
        userService.abandonLifecycleCommand(reviewed);
        const reloaded = await refreshUserAfterLifecycleConflict(
          reviewed.user_id,
          currentVersionFromConflict(error),
        );
        setReviewedLifecycleCommand(null);
        setLifecycleReauthenticationRequired(false);
        setLifecycleError(
          finalSuperadminRequired
            ? ADMIN_TEXT.USERS_FINAL_SUPERADMIN_REQUIRED
            : reloaded
            ? ADMIN_TEXT.USERS_LIFECYCLE.CONFLICT_RELOADED
            : ADMIN_TEXT.USERS_LIFECYCLE.CONFLICT_RETRY,
        );
        return;
      }
      setLifecycleError(
        getErrorMessage(error, ADMIN_TEXT.USERS_LIFECYCLE.APPLY_ERROR),
      );
    } finally {
      setIsApplyingLifecycle(false);
    }
  }

  async function refreshUserAfterRoleConflict(
    userId: string,
    currentRevision: number | null,
  ): Promise<boolean> {
    try {
      const response = await userService.find(usersQuery);
      const refreshedUser = response.records.find((user) => user.id === userId);
      if (refreshedUser) {
        setUsers((current) =>
          current.map((user) =>
            user.id === refreshedUser.id ? refreshedUser : user,
          ),
        );
        setEditTarget((current) =>
          current?.id === userId ? refreshedUser : current,
        );
        usersResource.retry();
        return true;
      }
    } catch {
      // The conflict revision below still prevents a stale retry when reload fails.
    }

    if (currentRevision !== null) {
      setEditTarget((current) =>
        current?.id === userId
          ? { ...current, authorization_revision: currentRevision }
          : current,
      );
    }
    usersResource.retry();
    return false;
  }

  function abandonReviewedRoleCommand(): void {
    if (reviewedRoleCommand) {
      userService.abandonRoleCommand(
        reviewedRoleCommand.user_id,
        reviewedRoleCommand.command_id,
      );
    }
  }

  function closeRoleEditor(): void {
    abandonReviewedRoleCommand();
    setReviewedRoleCommand(null);
    setRoleDraft(null);
    setRoleReviewError(null);
    setRoleReauthenticationRequired(false);
    setEditTarget(null);
  }

  function returnToRoleEditor(): void {
    abandonReviewedRoleCommand();
    setReviewedRoleCommand(null);
    setRoleReviewError(null);
    setRoleReauthenticationRequired(false);
  }

  async function handleReviewRoles(
    userId: string,
    request: UserRoleCommandInput,
  ): Promise<void> {
    if (!canUpdateUser) return;
    setIsReviewingRole(true);
    setRoleReviewError(null);
    try {
      const reviewed = await userService.previewRoles(userId, request);
      setRoleDraft(reviewed.input);
      setReviewedRoleCommand(reviewed);
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        const finalSuperadminRequired = isFinalSuperadminConflict(err);
        const didReload = await refreshUserAfterRoleConflict(
          userId,
          currentRevisionFromConflict(err),
        );
        setRoleReviewError(
          finalSuperadminRequired
            ? ADMIN_TEXT.USERS_FINAL_SUPERADMIN_REQUIRED
            : didReload
            ? ADMIN_TEXT.USERS_ROLE_CONFLICT_RELOADED
            : ADMIN_TEXT.USERS_ROLE_CONFLICT_RETRY,
        );
        return;
      }
      setRoleReviewError(
        getErrorMessage(err, ADMIN_TEXT.USERS_EDIT_ROLE_ERROR),
      );
    } finally {
      setIsReviewingRole(false);
    }
  }

  async function handleApplyRoles(currentPassword: string): Promise<void> {
    const reviewed = reviewedRoleCommand;
    if (!reviewed || !canUpdateUser) return;

    if (Date.parse(reviewed.review.expires_at) <= Date.now()) {
      userService.abandonRoleCommand(reviewed.user_id, reviewed.command_id);
      setReviewedRoleCommand(null);
      setRoleReviewError(ADMIN_TEXT.USERS_ROLE_REVIEW.EXPIRED);
      return;
    }

    setRoleReviewError(null);
    if (roleReauthenticationRequired) {
      setIsReauthenticatingRole(true);
      try {
        await authService.reauthenticateRecoveryContact(currentPassword);
        const refreshed = await session.refresh({ forceRotation: true });
        if (!refreshed) throw new TypeError("session refresh did not yield a session");
        setRoleReauthenticationRequired(false);
      } catch (err) {
        setRoleReviewError(
          getErrorMessage(err, ADMIN_TEXT.USERS_ROLE_REVIEW.REAUTH_ERROR),
        );
        return;
      } finally {
        setIsReauthenticatingRole(false);
      }
    }

    setIsApplyingRole(true);
    try {
      const receipt = await userService.applyRoles(reviewed);
      setUsers((current) =>
        current.map((user) =>
          user.id === receipt.user_id
            ? {
                ...user,
                roles: receipt.snapshot.roles,
                authorization_revision: receipt.snapshot.revision,
              }
            : user,
        ),
      );
      setReviewedRoleCommand(null);
      setRoleDraft(null);
      setRoleReviewError(null);
      setRoleReauthenticationRequired(false);
      setEditTarget(null);
      notify.success(ADMIN_TEXT.USERS_EDIT_ROLE_SUCCESS);
      await refreshCapabilities();
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setRoleReauthenticationRequired(true);
        setRoleReviewError(ADMIN_TEXT.USERS_ROLE_REVIEW.REAUTH_REQUIRED);
        return;
      }
      if (err instanceof ApiError && err.status === 409) {
        const finalSuperadminRequired = isFinalSuperadminConflict(err);
        userService.abandonRoleCommand(reviewed.user_id, reviewed.command_id);
        const didReload = await refreshUserAfterRoleConflict(
          reviewed.user_id,
          currentRevisionFromConflict(err),
        );
        setReviewedRoleCommand(null);
        setRoleReviewError(
          finalSuperadminRequired
            ? ADMIN_TEXT.USERS_FINAL_SUPERADMIN_REQUIRED
            : didReload
            ? ADMIN_TEXT.USERS_ROLE_CONFLICT_RELOADED
            : ADMIN_TEXT.USERS_ROLE_CONFLICT_RETRY,
        );
        return;
      }
      setRoleReviewError(
        getErrorMessage(err, ADMIN_TEXT.USERS_EDIT_ROLE_ERROR),
      );
    } finally {
      setIsApplyingRole(false);
    }
  }

  async function handleReviewInvitation(
    input: UserInvitationCommandInput,
  ): Promise<InviteUserReviewResult> {
    if (!canCreateUser) return { ok: false, error: new Error("forbidden") };
    setIsReviewingInvitation(true);
    setInvitationError(null);
    try {
      const reviewed = await userService.previewInvitation(input);
      setReviewedInvitation(reviewed);
      return { ok: true };
    } catch (err) {
      if (!isProfileSchemaConflict(err) && !invalidProfileValueDefinitionID(err)) {
        setInvitationError(
          getErrorMessage(err, ADMIN_TEXT.USERS_INVITATION.PREVIEW_ERROR),
        );
      }
      return { ok: false, error: err };
    } finally {
      setIsReviewingInvitation(false);
    }
  }

  function closeInvitationDialog(): void {
    if (reviewedInvitation) {
      userService.abandonInvitationCommand(reviewedInvitation);
    }
    setCreateOpen(false);
    setReviewedInvitation(null);
    setInvitationError(null);
    setInvitationReauthenticationRequired(false);
  }

  function returnToInvitationDraft(): void {
    if (reviewedInvitation) {
      userService.abandonInvitationCommand(reviewedInvitation);
    }
    setReviewedInvitation(null);
    setInvitationError(null);
    setInvitationReauthenticationRequired(false);
  }

  async function handleApplyInvitation(currentPassword: string): Promise<void> {
    const reviewed = reviewedInvitation;
    if (!reviewed || !canCreateUser) return;
    if (Date.parse(reviewed.review.expires_at) <= Date.now()) {
      userService.abandonInvitationCommand(reviewed);
      setReviewedInvitation(null);
      setInvitationReauthenticationRequired(false);
      setInvitationError(ADMIN_TEXT.USERS_INVITATION.EXPIRED);
      return;
    }

    setInvitationError(null);
    if (invitationReauthenticationRequired) {
      setIsReauthenticatingInvitation(true);
      try {
        await authService.reauthenticateRecoveryContact(currentPassword);
        const refreshed = await session.refresh({ forceRotation: true });
        if (!refreshed) {
          throw new TypeError("session refresh did not yield a session");
        }
        setInvitationReauthenticationRequired(false);
      } catch (error) {
        setInvitationError(
          getErrorMessage(error, ADMIN_TEXT.USERS_INVITATION.REAUTH_ERROR),
        );
        return;
      } finally {
        setIsReauthenticatingInvitation(false);
      }
    }

    setIsApplyingInvitation(true);
    try {
      await userService.applyInvitation(reviewed);
      const queryWillChange = page !== 1 || debouncedSearch !== "";
      setSearch("");
      setPage(1);
      if (!queryWillChange) usersResource.retry();
      setCreateOpen(false);
      setReviewedInvitation(null);
      setInvitationError(null);
      setInvitationReauthenticationRequired(false);
      notify.success(ADMIN_TEXT.USERS_INVITATION.SUCCESS);
    } catch (error) {
      if (isRecentAuthenticationRequired(error)) {
        setInvitationReauthenticationRequired(true);
        setInvitationError(ADMIN_TEXT.USERS_INVITATION.REAUTH_REQUIRED);
        return;
      }
      if (error instanceof ApiError && error.status === 409) {
        userService.abandonInvitationCommand(reviewed);
        setReviewedInvitation(null);
        setInvitationReauthenticationRequired(false);
        rolesResource.retry();
        setInvitationError(ADMIN_TEXT.USERS_INVITATION.CONFLICT);
        return;
      }
      setInvitationError(
        getErrorMessage(error, ADMIN_TEXT.USERS_INVITATION.APPLY_ERROR),
      );
    } finally {
      setIsApplyingInvitation(false);
    }
  }

  function openInvitationResendDialog(target: AdminUser): void {
    setInvitationResendTarget(target);
    setReviewedInvitationResend(null);
    setInvitationResendError(null);
    setInvitationResendReauthenticationRequired(false);
  }

  function closeInvitationResendDialog(): void {
    if (reviewedInvitationResend) {
      userService.abandonInvitationResendCommand(reviewedInvitationResend);
    }
    setInvitationResendTarget(null);
    setReviewedInvitationResend(null);
    setInvitationResendError(null);
    setInvitationResendReauthenticationRequired(false);
  }

  function returnToInvitationResendDraft(): void {
    if (reviewedInvitationResend) {
      userService.abandonInvitationResendCommand(reviewedInvitationResend);
    }
    setReviewedInvitationResend(null);
    setInvitationResendError(null);
    setInvitationResendReauthenticationRequired(false);
  }

  async function reloadInvitationResendTarget(userID: string): Promise<void> {
    try {
      const response = await userService.find(usersQuery);
      const refreshed = response.records.find((user) => user.id === userID);
      setUsers(response.records);
      setPagination(response.pagination);
      setInvitationResendTarget(
        refreshed?.status === "invited" ? refreshed : null,
      );
    } catch {
      usersResource.retry();
    }
  }

  async function handleReviewInvitationResend(reason: string): Promise<void> {
    const target = invitationResendTarget;
    if (!target || target.status !== "invited" || !canCreateUser) return;

    setIsReviewingInvitationResend(true);
    setInvitationResendError(null);
    try {
      const reviewed = await userService.previewInvitationResend(target, reason);
      setReviewedInvitationResend(reviewed);
    } catch (error) {
      if (error instanceof ApiError && error.status === 429) {
        setInvitationResendError(
          ADMIN_TEXT.USERS_INVITATION_RESEND.RATE_LIMITED,
        );
      } else if (error instanceof ApiError && error.status === 409) {
        await reloadInvitationResendTarget(target.id);
        setInvitationResendError(ADMIN_TEXT.USERS_INVITATION_RESEND.CONFLICT);
      } else {
        setInvitationResendError(
          getErrorMessage(
            error,
            ADMIN_TEXT.USERS_INVITATION_RESEND.PREVIEW_ERROR,
          ),
        );
      }
    } finally {
      setIsReviewingInvitationResend(false);
    }
  }

  async function handleApplyInvitationResend(
    currentPassword: string,
  ): Promise<void> {
    const reviewed = reviewedInvitationResend;
    if (!reviewed || !canCreateUser) return;

    if (Date.parse(reviewed.review.expires_at) <= Date.now()) {
      userService.abandonInvitationResendCommand(reviewed);
      setReviewedInvitationResend(null);
      setInvitationResendReauthenticationRequired(false);
      setInvitationResendError(ADMIN_TEXT.USERS_INVITATION_RESEND.EXPIRED);
      return;
    }

    setInvitationResendError(null);
    if (invitationResendReauthenticationRequired) {
      setIsReauthenticatingInvitationResend(true);
      try {
        await authService.reauthenticateRecoveryContact(currentPassword);
        const refreshed = await session.refresh({ forceRotation: true });
        if (!refreshed) {
          throw new TypeError("session refresh did not yield a session");
        }
        setInvitationResendReauthenticationRequired(false);
      } catch (error) {
        setInvitationResendError(
          getErrorMessage(
            error,
            ADMIN_TEXT.USERS_INVITATION_RESEND.REAUTH_ERROR,
          ),
        );
        return;
      } finally {
        setIsReauthenticatingInvitationResend(false);
      }
    }

    setIsApplyingInvitationResend(true);
    try {
      await userService.applyInvitationResend(reviewed);
      setInvitationResendTarget(null);
      setReviewedInvitationResend(null);
      setInvitationResendError(null);
      setInvitationResendReauthenticationRequired(false);
      notify.success(ADMIN_TEXT.USERS_INVITATION_RESEND.SUCCESS);
    } catch (error) {
      if (isRecentAuthenticationRequired(error)) {
        setInvitationResendReauthenticationRequired(true);
        setInvitationResendError(
          ADMIN_TEXT.USERS_INVITATION_RESEND.REAUTH_REQUIRED,
        );
        return;
      }
      if (error instanceof ApiError && error.status === 409) {
        userService.abandonInvitationResendCommand(reviewed);
        setReviewedInvitationResend(null);
        setInvitationResendReauthenticationRequired(false);
        await reloadInvitationResendTarget(reviewed.user_id);
        setInvitationResendError(ADMIN_TEXT.USERS_INVITATION_RESEND.CONFLICT);
        return;
      }
      if (error instanceof ApiError && error.status === 429) {
        setInvitationResendError(
          ADMIN_TEXT.USERS_INVITATION_RESEND.RATE_LIMITED,
        );
        return;
      }
      setInvitationResendError(
        getErrorMessage(
          error,
          ADMIN_TEXT.USERS_INVITATION_RESEND.APPLY_ERROR,
        ),
      );
    } finally {
      setIsApplyingInvitationResend(false);
    }
  }

  const columns: AdminColumn<AdminUser>[] = [
    {
      key: "id",
      label: ADMIN_TEXT.USERS_COL_ID,
      className: "w-32",
      render: (u) => (
        <span className="font-mono text-xs tabular-nums text-muted-foreground" title={u.id}>
          {u.id.slice(0, 8)}…
        </span>
      ),
    },
    {
      key: "username",
      label: ADMIN_TEXT.USERS_COL_USERNAME,
      render: (u) => <span className="font-medium">{u.username}</span>,
    },
    {
      key: "status",
      label: ADMIN_TEXT.USERS_COL_STATUS,
      render: (u) => (
        <Badge
          variant="outline"
          className={
            u.status === "active"
              ? "border-success/30 bg-success/10 text-success"
              : u.status === "suspended"
                ? "border-warning/30 bg-warning/10 text-warning"
                : u.status === "invited"
                  ? "border-info/30 bg-info/10 text-info"
                  : "border-destructive/30 bg-destructive/10 text-destructive"
          }
        >
          {ADMIN_TEXT.USERS_LIFECYCLE.STATUS(u.status)}
        </Badge>
      ),
    },
    {
      key: "role",
      label: ADMIN_TEXT.USERS_COL_ROLE,
      render: (u) => (
        <div className="flex flex-wrap gap-1.5">
          {u.roles.length > 0 ? (
            u.roles.map((roleKey) => {
              const role = roles.find((candidate) => candidate.key === roleKey);
              const roleName = role ? roleDisplayName(role) : roleKey;
              return (
                <Badge
                  key={roleKey}
                  variant="outline"
                  className={`border text-[11px] ${ROLE_COLORS[roleKey.toLowerCase()] ?? "bg-muted text-muted-foreground"}`}
                >
                  {roleName}
                </Badge>
              );
            })
          ) : (
            <span className="text-xs text-muted-foreground">
              {ADMIN_TEXT.USERS_NO_ROLES}
            </span>
          )}
        </div>
      ),
    },
    {
      key: "created_at",
      label: ADMIN_TEXT.USERS_COL_CREATED,
      render: (u) => <span className="text-xs text-muted-foreground">{u.created_at?.slice(0, 10)}</span>,
    },
    {
      key: "actions",
      label: ADMIN_TEXT.ACTIONS,
      className: "w-60",
      render: (u) => (
        <div className="flex items-center gap-1 justify-end">
          {canUpdateUser ? (
            <Button
              variant="ghost" size="icon"
              className="h-11 w-11 text-muted-foreground transition-colors hover:text-foreground"
              onClick={() => {
                setRoleDraft(null);
                setRoleReviewError(null);
                setRoleReauthenticationRequired(false);
                setEditTarget(u);
              }}
              disabled={
                rolesResource.status !== "ready" ||
                roles.length === 0 ||
                isReviewingRole ||
                isApplyingRole ||
                isReauthenticatingRole ||
                isReviewingLifecycle ||
                isApplyingLifecycle ||
                isReauthenticatingLifecycle
              }
              aria-label={ADMIN_TEXT.USERS_EDIT_ROLES_ACTION(u.username)}
            >
              <UserCog className="h-4 w-4" aria-hidden="true" />
            </Button>
          ) : null}
          {canCreateUser && u.status === "invited" ? (
            <Button
              variant="ghost"
              size="icon"
              className="h-11 w-11 text-muted-foreground transition-colors hover:text-info"
              onClick={() => openInvitationResendDialog(u)}
              disabled={
                isReviewingInvitationResend ||
                isApplyingInvitationResend ||
                isReauthenticatingInvitationResend ||
                isReviewingInvitation ||
                isApplyingInvitation ||
                isReauthenticatingInvitation
              }
              aria-label={ADMIN_TEXT.USERS_INVITATION_RESEND.ACTION(
                u.username,
              )}
            >
              <RefreshCw className="size-4" aria-hidden="true" />
            </Button>
          ) : null}
          {canUpdateUser && u.id !== currentUser?.id && u.status === "active" ? (
            <Button
              variant="ghost"
              size="icon"
              className="h-11 w-11 text-muted-foreground transition-colors hover:text-warning"
              onClick={() => openLifecycleDialog(u, "suspend")}
              disabled={
                isReviewingLifecycle ||
                isApplyingLifecycle ||
                isReauthenticatingLifecycle
              }
              aria-label={ADMIN_TEXT.USERS_LIFECYCLE.SUSPEND_ACTION(u.username)}
            >
              <PauseCircle className="size-4" aria-hidden="true" />
            </Button>
          ) : null}
          {canUpdateUser &&
          u.id !== currentUser?.id &&
          u.status === "suspended" ? (
            <Button
              variant="ghost"
              size="icon"
              className="h-11 w-11 text-muted-foreground transition-colors hover:text-success"
              onClick={() => openLifecycleDialog(u, "reactivate")}
              disabled={
                isReviewingLifecycle ||
                isApplyingLifecycle ||
                isReauthenticatingLifecycle
              }
              aria-label={ADMIN_TEXT.USERS_LIFECYCLE.REACTIVATE_ACTION(
                u.username,
              )}
            >
              <PlayCircle className="size-4" aria-hidden="true" />
            </Button>
          ) : null}
          {canDeleteUser && u.id !== currentUser?.id ? (
            <Button
              variant="ghost" size="icon"
              className="h-11 w-11 text-muted-foreground transition-colors hover:text-destructive"
              onClick={() => openLifecycleDialog(u, "delete")}
              disabled={
                isReviewingRole ||
                isApplyingRole ||
                isReauthenticatingRole ||
                isReviewingLifecycle ||
                isApplyingLifecycle ||
                isReauthenticatingLifecycle
              }
              aria-label={ADMIN_TEXT.USERS_LIFECYCLE.DELETE_ACTION(u.username)}
            >
              <Trash2 className="h-4 w-4" aria-hidden="true" />
            </Button>
          ) : null}
        </div>
      ),
    },
  ];

  const isUsersLoading = usersResource.status === "loading";
  const usersLoadError = usersResource.status === "error";
  const usersBelongToCurrentQuery = !usersResource.isPreviousData;
  const visibleUsers = usersBelongToCurrentQuery ? users : [];
  const visiblePagination = usersBelongToCurrentQuery
    ? pagination
    : undefined;
  const isRoleCatalogReady =
    rolesResource.status === "ready" && roles.length > 0;

  if (isUsersLoading && visibleUsers.length === 0) {
    return (
      <div
        className="flex h-[50vh] items-center justify-center"
        role="status"
        aria-label={ADMIN_TEXT.USERS_LOADING}
      >
        <Loader2
          className="h-8 w-8 animate-spin text-primary motion-reduce:animate-none"
          aria-hidden="true"
        />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{ADMIN_TEXT.USERS_TITLE}</h1>
          <p className="text-sm text-muted-foreground mt-1">{ADMIN_TEXT.USERS_SUBTITLE}</p>
        </div>
        {canCreateUser ? (
          <Button
            className="min-h-11 gap-2 cursor-pointer"
            onClick={() => setCreateOpen(true)}
            disabled={!isRoleCatalogReady}
          >
            <Plus className="h-4 w-4" aria-hidden="true" />
            {ADMIN_TEXT.USERS_CREATE}
          </Button>
        ) : null}
      </div>

      {canManageUsers && rolesResource.status !== "ready" ? (
        <div
          className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-warning/25 bg-warning/10 px-4 py-3 text-sm"
          role={rolesResource.status === "error" ? "alert" : "status"}
        >
          <div className="flex min-w-0 items-center gap-2 text-warning">
            {rolesResource.status === "loading" ? (
              <Loader2
                className="size-4 shrink-0 animate-spin motion-reduce:animate-none"
                aria-hidden="true"
              />
            ) : (
              <CircleAlert className="size-4 shrink-0" aria-hidden="true" />
            )}
            <span>
              {rolesResource.status === "loading"
                ? ADMIN_TEXT.USERS_ROLE_CATALOG_LOADING
                : ADMIN_TEXT.USERS_ROLE_CATALOG_ERROR}
            </span>
          </div>
          {rolesResource.status === "error" ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={rolesResource.retry}
              className="min-h-9 gap-2 bg-background/70"
            >
              <RefreshCw className="size-4" aria-hidden="true" />
              {TEXT.COMMON.RETRY}
            </Button>
          ) : null}
        </div>
      ) : null}

      {canManageUsers && rolesResource.status === "ready" && roles.length === 0 ? (
        <div
          className="rounded-xl border border-warning/25 bg-warning/10 px-4 py-3 text-sm text-foreground"
          role="alert"
        >
          {ADMIN_TEXT.USERS_ROLE_CATALOG_EMPTY}
        </div>
      ) : null}

      <div className="relative max-w-sm">
        <Search
          className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
          aria-hidden="true"
        />
        <Input
          value={search}
          onChange={(event) => {
            setSearch(event.target.value);
            setPage(1);
          }}
          placeholder={ADMIN_TEXT.USERS_SEARCH_PLACEHOLDER}
          className="min-h-11 pl-9"
          aria-label={ADMIN_TEXT.USERS_SEARCH_PLACEHOLDER}
        />
      </div>

      {usersLoadError && visibleUsers.length === 0 ? (
        <div
          className="flex min-h-64 flex-col items-center justify-center rounded-xl border border-destructive/20 bg-destructive/5 px-6 text-center"
          role="alert"
        >
          <CircleAlert className="mb-4 size-8 text-destructive" aria-hidden="true" />
          <p className="max-w-md text-sm text-muted-foreground">
            {ADMIN_TEXT.USERS_LOAD_ERROR}
          </p>
          <Button type="button" variant="outline" onClick={usersResource.retry} className="mt-5 min-h-11 gap-2">
            <RefreshCw className="size-4" aria-hidden="true" />
            {TEXT.COMMON.RETRY}
          </Button>
        </div>
      ) : (
        <div className="space-y-3">
          {usersLoadError ? (
            <div
              className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-destructive/20 bg-destructive/5 px-4 py-3 text-sm"
              role="alert"
            >
              <span className="text-muted-foreground">
                {ADMIN_TEXT.USERS_REFRESH_ERROR}
              </span>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={usersResource.retry}
                className="min-h-9 gap-2"
              >
                <RefreshCw className="size-4" aria-hidden="true" />
                {TEXT.COMMON.RETRY}
              </Button>
            </div>
          ) : null}
          <div className={isUsersLoading ? "pointer-events-none opacity-60" : undefined} aria-busy={isUsersLoading}>
          <AdminDataTable
            columns={columns}
            data={visibleUsers}
            keyExtractor={(u) => u.id}
            emptyMessage={ADMIN_TEXT.USERS_EMPTY}
            pagination={visiblePagination}
            onPageChange={setPage}
          />
          </div>
        </div>
      )}

      {(lifecycleAction === "delete" ? canDeleteUser : canUpdateUser) ? (
        <UserLifecycleDialog
          target={lifecycleTarget}
          action={lifecycleAction}
          reviewed={reviewedLifecycleCommand}
          reviewing={isReviewingLifecycle}
          applying={isApplyingLifecycle}
          reauthenticating={isReauthenticatingLifecycle}
          requiresReauthentication={lifecycleReauthenticationRequired}
          operationError={lifecycleError}
          onReview={handleReviewLifecycle}
          onApply={handleApplyLifecycle}
          onBack={returnToLifecycleEditor}
          onClose={closeLifecycleDialog}
        />
      ) : null}

      {canUpdateUser && isRoleCatalogReady && !reviewedRoleCommand ? (
        <EditRoleDialog
          key={`${editTarget?.id ?? "closed"}:${editTarget?.authorization_revision ?? 0}:${roleDraft?.expected_revision ?? 0}`}
          user={editTarget}
          roles={roles}
          initialRequest={roleDraft}
          onReview={handleReviewRoles}
          onClose={closeRoleEditor}
          isReviewing={isReviewingRole}
          operationError={roleReviewError}
        />
      ) : null}

      {canUpdateUser ? (
        <UserRoleReviewDialog
          open={!!reviewedRoleCommand}
          user={editTarget}
          roles={roles}
          reviewed={reviewedRoleCommand}
          applying={isApplyingRole}
          reauthenticating={isReauthenticatingRole}
          requiresReauthentication={roleReauthenticationRequired}
          operationError={roleReviewError}
          onBack={returnToRoleEditor}
          onClose={closeRoleEditor}
          onConfirm={handleApplyRoles}
        />
      ) : null}

      {canCreateUser && isRoleCatalogReady ? (
        <InviteUserDialog
          open={createOpen}
          roles={roles}
          reviewed={reviewedInvitation}
          isReviewing={isReviewingInvitation}
          isApplying={isApplyingInvitation}
          isReauthenticating={isReauthenticatingInvitation}
          requiresReauthentication={invitationReauthenticationRequired}
          operationError={invitationError}
          onReview={handleReviewInvitation}
          onApply={handleApplyInvitation}
          onBack={returnToInvitationDraft}
          onClose={closeInvitationDialog}
        />
      ) : null}

      {canCreateUser ? (
        <UserInvitationResendDialog
          target={invitationResendTarget}
          reviewed={reviewedInvitationResend}
          reviewing={isReviewingInvitationResend}
          applying={isApplyingInvitationResend}
          reauthenticating={isReauthenticatingInvitationResend}
          requiresReauthentication={
            invitationResendReauthenticationRequired
          }
          operationError={invitationResendError}
          onReview={handleReviewInvitationResend}
          onApply={handleApplyInvitationResend}
          onBack={returnToInvitationResendDraft}
          onClose={closeInvitationResendDialog}
        />
      ) : null}
    </div>
  );
}
