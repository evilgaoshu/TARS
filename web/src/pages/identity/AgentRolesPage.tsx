import { useCallback, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  RegistryDetail,
  RegistrySidebar,
  RegistryCard,
  RegistryPanel,
  SplitLayout,
} from "@/components/ui/registry-primitives";
import {
  createAgentRole,
  deleteAgentRole,
  fetchAgentRoles,
  setAgentRoleEnabled,
  updateAgentRole,
} from "../../lib/api/agent-roles";
import type {
  AgentRole,
  AgentRoleModelBinding,
  AgentRoleModelTargetBinding,
} from "../../lib/api/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { NativeSelect } from "@/components/ui/select";
import { DetailHeader } from "@/components/ui/detail-header";
import { EmptyDetailState } from "@/components/ui/empty-detail-state";
import { SectionTitle, SummaryGrid, StatCard } from "@/components/ui/page-hero";
import { ActiveBadge as StatusBadge } from "@/components/ui/active-badge";
import { useNotify } from "@/hooks/ui/useNotify";
import { useI18n } from "@/hooks/useI18n";
import {
  Plus,
  Bot,
  Shield,
  Zap,
  Edit3,
  Save,
  X,
  Trash2,
  Power,
} from "lucide-react";
import { clsx } from "clsx";

function normalizeModelBindingTarget(
  target?: AgentRoleModelTargetBinding,
): AgentRoleModelTargetBinding | undefined {
  if (!target?.provider_id && !target?.model) {
    return undefined;
  }
  return {
    provider_id: target.provider_id || "",
    model: target.model || "",
  };
}

function roleModelBinding(role?: Partial<AgentRole>): AgentRoleModelBinding {
  const primary = normalizeModelBindingTarget(role?.model_binding?.primary);
  const fallback = normalizeModelBindingTarget(role?.model_binding?.fallback);
  return {
    primary,
    fallback,
    inherit_platform_default:
      role?.model_binding?.inherit_platform_default ?? (!primary && !fallback),
  };
}

function parseCSV(value: string): string[] {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

const EMPTY_ROLE: Partial<AgentRole> = {
  role_id: "",
  display_name: "",
  description: "",
  status: "active",
  profile: { system_prompt: "", persona_tags: [] },
  capability_binding: { mode: "unrestricted" },
  policy_binding: { max_risk_level: "warning", max_action: "require_approval" },
  model_binding: { inherit_platform_default: true },
};

export const AgentRolesPage = () => {
  const queryClient = useQueryClient();
  const notify = useNotify();
  const { t } = useI18n();
  const [selectedID, setSelectedID] = useState("");
  const [isCreating, setIsCreating] = useState(false);
  const [panelMode, setPanelMode] = useState<"detail" | "edit">("detail");
  const [formData, setFormData] = useState<Partial<AgentRole>>(EMPTY_ROLE);

  const { data: rolesData, isLoading } = useQuery({
    queryKey: ["agent-roles"],
    queryFn: () => fetchAgentRoles({ limit: 100 }),
  });

  const roles = rolesData?.items || [];
  const selectedRole = roles.find((r) => r.role_id === selectedID);

  const builtinCount = roles.filter((r) => r.is_builtin).length;
  const customCount = roles.filter((r) => !r.is_builtin).length;
  const enabledCount = roles.filter((r) => r.status === "active").length;

  const createMutation = useMutation({
    mutationFn: (data: Partial<AgentRole>) => createAgentRole(data),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ["agent-roles"] });
      setSelectedID(data.role_id);
      setIsCreating(false);
      setPanelMode("detail");
      notify.success(t("status.success"));
    },
    onError: (err) => notify.error(err, t("status.error")),
  });

  const updateMutation = useMutation({
    mutationFn: (data: Partial<AgentRole>) => updateAgentRole(selectedID, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["agent-roles"] });
      setPanelMode("detail");
      notify.success(t("status.success"));
    },
    onError: (err) => notify.error(err, t("status.error")),
  });

  const deleteMutation = useMutation({
    mutationFn: (roleID: string) => deleteAgentRole(roleID),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["agent-roles"] });
      setSelectedID("");
      setPanelMode("detail");
      notify.success(t("status.success"));
    },
    onError: (err) => notify.error(err, t("status.error")),
  });

  const toggleMutation = useMutation({
    mutationFn: ({ roleID, enabled }: { roleID: string; enabled: boolean }) =>
      setAgentRoleEnabled(roleID, enabled),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["agent-roles"] });
      notify.success(t("status.success"));
    },
    onError: (err) => notify.error(err, t("status.error")),
  });

  const handleSelect = useCallback((role: AgentRole) => {
    setSelectedID(role.role_id);
    setIsCreating(false);
    setPanelMode("detail");
  }, []);

  const startCreate = () => {
    setIsCreating(true);
    setSelectedID("");
    setPanelMode("edit");
    setFormData({ ...EMPTY_ROLE });
  };

  const startEdit = () => {
    if (!selectedRole) return;
    setFormData({ ...selectedRole });
    setPanelMode("edit");
  };

  const onSave = () => {
    const payload = {
      ...formData,
      model_binding: roleModelBinding(formData),
    };
    if (isCreating) createMutation.mutate(payload);
    else updateMutation.mutate(payload);
  };

  const updateForm = (patch: Partial<AgentRole>) => {
    setFormData((prev) => ({ ...prev, ...patch }));
  };

  return (
    <div className="animate-fade-in flex flex-col gap-6">
      <SectionTitle
        title={t("identity.agentRoles.title")}
        subtitle={t("identity.agentRoles.subtitle")}
      />

      <SummaryGrid>
        <StatCard
          title={t("identity.agentRoles.stats.total")}
          value={String(roles.length)}
          subtitle={t("identity.agentRoles.stats.totalDesc")}
          icon={<Bot size={16} />}
        />
        <StatCard
          title={t("identity.agentRoles.stats.builtin")}
          value={String(builtinCount)}
          subtitle={t("identity.agentRoles.stats.builtinDesc")}
          icon={<Shield size={16} />}
        />
        <StatCard
          title={t("identity.agentRoles.stats.custom")}
          value={String(customCount)}
          subtitle={t("identity.agentRoles.stats.customDesc")}
        />
        <StatCard
          title={t("identity.agentRoles.stats.enabled")}
          value={String(enabledCount)}
          subtitle={t("identity.agentRoles.stats.enabledDesc")}
          icon={<Zap size={16} />}
        />
      </SummaryGrid>

      <SplitLayout
        sidebar={
          <RegistrySidebar key="agent-roles-sidebar">
            <div className="flex justify-between items-center mb-4 p-4 pb-0">
              <h2 className="text-lg font-bold m-0 text-foreground">
                {t("identity.agentRoles.sidebar.title")}
              </h2>
              <Button size="sm" onClick={startCreate} variant="amber">
                <Plus size={14} /> {t("identity.agentRoles.sidebar.new")}
              </Button>
            </div>
            {isLoading ? (
              <div className="p-10 text-center text-muted-foreground animate-pulse">
                {t("common.loading")}
              </div>
            ) : (
              <div className="px-4">
                <RegistryPanel
                  title={t("identity.agentRoles.sidebar.panelBuiltin")}
                  emptyText={t("identity.agentRoles.sidebar.empty")}
                >
                  <div className="flex flex-col gap-2">
                    {roles
                      .filter((r) => r.is_builtin)
                      .map((item) => (
                        <button
                          key={item.role_id}
                          onClick={() => handleSelect(item)}
                          className={clsx(
                            "text-left w-full focus:outline-none rounded-2xl transition-all",
                            selectedID === item.role_id
                              ? "ring-2 ring-primary"
                              : "",
                          )}
                        >
                          <RegistryCard
                            title={item.display_name || item.role_id}
                            subtitle={item.role_id}
                            lines={[
                              `${t("common.mode")}: ${item.capability_binding?.mode || t("common.na")} | ${t("common.maxRisk")}: ${item.policy_binding?.max_risk_level || t("common.na")}`,
                            ]}
                            status={
                              <StatusBadge
                                active={item.status === "active"}
                                label={item.status || "active"}
                              />
                            }
                          />
                        </button>
                      ))}
                  </div>
                </RegistryPanel>
                <div className="h-6" />
                <RegistryPanel
                  title={t("identity.agentRoles.sidebar.panelCustom")}
                  emptyText={t("identity.agentRoles.sidebar.empty")}
                >
                  <div className="flex flex-col gap-2">
                    {roles
                      .filter((r) => !r.is_builtin)
                      .map((item) => (
                        <button
                          key={item.role_id}
                          onClick={() => handleSelect(item)}
                          className={clsx(
                            "text-left w-full focus:outline-none rounded-2xl transition-all",
                            selectedID === item.role_id
                              ? "ring-2 ring-primary"
                              : "",
                          )}
                        >
                          <RegistryCard
                            title={item.display_name || item.role_id}
                            subtitle={item.role_id}
                            lines={[
                              `${t("common.mode")}: ${item.capability_binding?.mode || t("common.na")} | ${t("common.maxRisk")}: ${item.policy_binding?.max_risk_level || t("common.na")}`,
                            ]}
                            status={
                              <StatusBadge
                                active={item.status === "active"}
                                label={item.status || "active"}
                              />
                            }
                          />
                        </button>
                      ))}
                  </div>
                </RegistryPanel>
              </div>
            )}
          </RegistrySidebar>
        }
        detail={
          isCreating || selectedRole ? (
            <RegistryDetail key={selectedID || "new"}>
              <DetailHeader
                title={
                  isCreating
                    ? t("identity.agentRoles.detail.create")
                    : selectedRole?.display_name ||
                      selectedRole?.role_id ||
                      t("identity.agentRoles.detail.title")
                }
                subtitle={
                  isCreating
                    ? t("identity.agentRoles.detail.provision")
                    : selectedID
                }
                status={
                  !isCreating &&
                  selectedRole && (
                    <StatusBadge
                      active={selectedRole.status === "active"}
                      label={selectedRole.status || "active"}
                    />
                  )
                }
                actions={
                  !isCreating &&
                  selectedRole &&
                  panelMode === "detail" && (
                    <div className="flex gap-2">
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() =>
                          toggleMutation.mutate({
                            roleID: selectedID,
                            enabled: selectedRole.status !== "active",
                          })
                        }
                        disabled={toggleMutation.isPending}
                      >
                        <Power size={14} />{" "}
                        {selectedRole.status === "active"
                          ? t("action.disable")
                          : t("action.enable")}
                      </Button>
                      <Button variant="outline" size="sm" onClick={startEdit}>
                        <Edit3 size={14} />{" "}
                        {t("identity.agentRoles.detail.edit")}
                      </Button>
                      {!selectedRole.is_builtin && (
                        <Button
                          variant="destructive"
                          size="sm"
                          onClick={() => {
                            if (
                              confirm(
                                t("identity.agentRoles.detail.deleteConfirm"),
                              )
                            )
                              deleteMutation.mutate(selectedID);
                          }}
                          disabled={deleteMutation.isPending}
                        >
                          <Trash2 size={14} />{" "}
                          {t("identity.agentRoles.detail.delete")}
                        </Button>
                      )}
                    </div>
                  )
                }
              />

              <div className="p-6">
                {panelMode === "edit" || isCreating ? (
                  <div className="space-y-6">
                    {/* Basic Info */}
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                      <div>
                        <label className="text-xs font-bold text-muted-foreground uppercase mb-1 block">
                          {t("identity.agentRoles.detail.roleId")}
                        </label>
                        <Input
                          value={formData.role_id || ""}
                          onChange={(e) =>
                            updateForm({ role_id: e.target.value })
                          }
                          placeholder={t(
                            "identity.agentRoles.detail.roleIdPlaceholder",
                            "e.g. diagnosis",
                          )}
                          disabled={!isCreating}
                        />
                      </div>
                      <div>
                        <label className="text-xs font-bold text-muted-foreground uppercase mb-1 block">
                          {t("identity.agentRoles.detail.displayName")}
                        </label>
                        <Input
                          value={formData.display_name || ""}
                          onChange={(e) =>
                            updateForm({ display_name: e.target.value })
                          }
                          placeholder={t(
                            "identity.agentRoles.detail.displayNamePlaceholder",
                            "e.g. Diagnosis Agent",
                          )}
                        />
                      </div>
                    </div>
                    <div>
                      <label className="text-xs font-bold text-muted-foreground uppercase mb-1 block">
                        {t("identity.agentRoles.detail.description")}
                      </label>
                      <Input
                        value={formData.description || ""}
                        onChange={(e) =>
                          updateForm({ description: e.target.value })
                        }
                        placeholder={t(
                          "identity.agentRoles.detail.descriptionPlaceholder",
                          "What this role does",
                        )}
                      />
                    </div>

                    {/* Profile */}
                    <div className="space-y-3">
                      <h3 className="text-sm font-bold text-muted-foreground uppercase tracking-widest">
                        {t("identity.agentRoles.detail.profile")}
                      </h3>
                      <div>
                        <label className="text-xs font-bold text-muted-foreground mb-1 block">
                          {t("identity.agentRoles.detail.systemPrompt")}
                        </label>
                        <textarea
                          className="w-full min-h-[80px] rounded-lg border border-border bg-background px-3 py-2 text-sm focus:ring-2 focus:ring-primary/20 transition-all outline-none"
                          value={formData.profile?.system_prompt || ""}
                          onChange={(e) =>
                            updateForm({
                              profile: {
                                ...formData.profile!,
                                system_prompt: e.target.value,
                              },
                            })
                          }
                          placeholder={t(
                            "identity.agentRoles.detail.systemPromptPlaceholder",
                            "Agent system prompt...",
                          )}
                        />
                      </div>
                      <div>
                        <label className="text-xs font-bold text-muted-foreground mb-1 block">
                          {t("identity.agentRoles.detail.personaTags")}
                        </label>
                        <Input
                          value={(formData.profile?.persona_tags || []).join(
                            ", ",
                          )}
                          onChange={(e) =>
                            updateForm({
                              profile: {
                                ...formData.profile!,
                                persona_tags: e.target.value
                                  .split(",")
                                  .map((s) => s.trim())
                                  .filter(Boolean),
                              },
                            })
                          }
                          placeholder={t(
                            "identity.agentRoles.detail.personaTagsPlaceholder",
                            "cautious, read-only, sre",
                          )}
                        />
                      </div>
                    </div>

                    {/* Capability Binding */}
                    <div className="space-y-3">
                      <h3 className="text-sm font-bold text-muted-foreground uppercase tracking-widest">
                        {t("identity.agentRoles.detail.capabilityBinding")}
                      </h3>
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <div>
                          <label className="text-xs font-bold text-muted-foreground mb-1 block">
                            {t("identity.agentRoles.detail.capMode")}
                          </label>
                          <NativeSelect
                            value={
                              formData.capability_binding?.mode ||
                              "unrestricted"
                            }
                            onChange={(e) =>
                              updateForm({
                                capability_binding: {
                                  ...formData.capability_binding!,
                                  mode: e.target.value,
                                },
                              })
                            }
                          >
                            <option value="unrestricted">
                              {t("identity.agentRoles.detail.modeUnrestricted")}
                            </option>
                            <option value="whitelist">
                              {t("identity.agentRoles.detail.modeWhitelist")}
                            </option>
                            <option value="blacklist">
                              {t("identity.agentRoles.detail.modeBlacklist")}
                            </option>
                          </NativeSelect>
                        </div>
                        <div>
                          <label className="text-xs font-bold text-muted-foreground mb-1 block">
                            {t("identity.agentRoles.detail.allowedSkillsCSV")}
                          </label>
                          <Input
                            value={(
                              formData.capability_binding?.allowed_skills || []
                            ).join(", ")}
                            onChange={(e) =>
                              updateForm({
                                capability_binding: {
                                  ...formData.capability_binding!,
                                  allowed_skills: parseCSV(e.target.value),
                                },
                              })
                            }
                            placeholder={t(
                              "identity.agentRoles.detail.allowedSkillsPlaceholder",
                              "skill_a, skill_b",
                            )}
                          />
                        </div>
                        <div>
                          <label className="text-xs font-bold text-muted-foreground mb-1 block">
                            {t(
                              "identity.agentRoles.detail.allowedSkillTags",
                              "Allowed Skill Tags",
                            )}
                          </label>
                          <Input
                            value={(
                              formData.capability_binding?.allowed_skill_tags ||
                              []
                            ).join(", ")}
                            onChange={(e) =>
                              updateForm({
                                capability_binding: {
                                  ...formData.capability_binding!,
                                  allowed_skill_tags: parseCSV(e.target.value),
                                },
                              })
                            }
                            placeholder={t(
                              "identity.agentRoles.detail.allowedSkillTagsPlaceholder",
                              "storage, kubernetes",
                            )}
                          />
                        </div>
                        <div>
                          <label className="text-xs font-bold text-muted-foreground mb-1 block">
                            {t(
                              "identity.agentRoles.detail.allowedConnectorCapabilities",
                              "Allowed Connector Capabilities",
                            )}
                          </label>
                          <Input
                            value={(
                              formData.capability_binding
                                ?.allowed_connector_capabilities || []
                            ).join(", ")}
                            onChange={(e) =>
                              updateForm({
                                capability_binding: {
                                  ...formData.capability_binding!,
                                  allowed_connector_capabilities: parseCSV(
                                    e.target.value,
                                  ),
                                },
                              })
                            }
                            placeholder={t(
                              "identity.agentRoles.detail.allowedConnectorCapabilitiesPlaceholder",
                              "metrics.query_instant, observability.query",
                            )}
                          />
                        </div>
                        <div>
                          <label className="text-xs font-bold text-muted-foreground mb-1 block">
                            {t(
                              "identity.agentRoles.detail.deniedConnectorCapabilities",
                              "Denied Connector Capabilities",
                            )}
                          </label>
                          <Input
                            value={(
                              formData.capability_binding
                                ?.denied_connector_capabilities || []
                            ).join(", ")}
                            onChange={(e) =>
                              updateForm({
                                capability_binding: {
                                  ...formData.capability_binding!,
                                  denied_connector_capabilities: parseCSV(
                                    e.target.value,
                                  ),
                                },
                              })
                            }
                            placeholder={t(
                              "identity.agentRoles.detail.deniedConnectorCapabilitiesPlaceholder",
                              "execution.run_command",
                            )}
                          />
                        </div>
                      </div>
                    </div>

                    {/* Policy Binding */}
                    <div className="space-y-3">
                      <h3 className="text-sm font-bold text-muted-foreground uppercase tracking-widest">
                        {t("identity.agentRoles.detail.policyBinding")}
                      </h3>
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <div>
                          <label className="text-xs font-bold text-muted-foreground mb-1 block">
                            {t("identity.agentRoles.detail.maxRisk")}
                          </label>
                          <NativeSelect
                            value={
                              formData.policy_binding?.max_risk_level ||
                              "warning"
                            }
                            onChange={(e) =>
                              updateForm({
                                policy_binding: {
                                  ...formData.policy_binding!,
                                  max_risk_level: e.target.value,
                                },
                              })
                            }
                          >
                            <option value="info">
                              {t("identity.agentRoles.detail.riskInfo")}
                            </option>
                            <option value="warning">
                              {t("identity.agentRoles.detail.riskWarning")}
                            </option>
                            <option value="critical">
                              {t("identity.agentRoles.detail.riskCritical")}
                            </option>
                          </NativeSelect>
                        </div>
                        <div>
                          <label className="text-xs font-bold text-muted-foreground mb-1 block">
                            {t("identity.agentRoles.detail.maxAction")}
                          </label>
                          <NativeSelect
                            value={
                              formData.policy_binding?.max_action ||
                              "require_approval"
                            }
                            onChange={(e) =>
                              updateForm({
                                policy_binding: {
                                  ...formData.policy_binding!,
                                  max_action: e.target.value,
                                },
                              })
                            }
                          >
                            <option value="suggest_only">
                              {t(
                                "identity.agentRoles.detail.actionSuggestOnly",
                              )}
                            </option>
                            <option value="require_approval">
                              {t(
                                "identity.agentRoles.detail.actionRequireApproval",
                              )}
                            </option>
                            <option value="direct_execute">
                              {t(
                                "identity.agentRoles.detail.actionDirectExecute",
                              )}
                            </option>
                          </NativeSelect>
                        </div>
                      </div>
                      <div>
                        <label className="text-xs font-bold text-muted-foreground mb-1 block">
                          {t("identity.agentRoles.detail.hardDeny")}
                        </label>
                        <Input
                          value={(
                            formData.policy_binding?.hard_deny || []
                          ).join(", ")}
                          onChange={(e) =>
                            updateForm({
                              policy_binding: {
                                ...formData.policy_binding!,
                                hard_deny: parseCSV(e.target.value),
                              },
                            })
                          }
                          placeholder={t(
                            "identity.agentRoles.detail.hardDenyPlaceholder",
                            "rm -rf, shutdown",
                          )}
                        />
                      </div>
                      <div>
                        <label className="text-xs font-bold text-muted-foreground mb-1 block">
                          {t(
                            "identity.agentRoles.detail.requireApprovalFor",
                            "Require Approval For",
                          )}
                        </label>
                        <Input
                          value={(
                            formData.policy_binding?.require_approval_for || []
                          ).join(", ")}
                          onChange={(e) =>
                            updateForm({
                              policy_binding: {
                                ...formData.policy_binding!,
                                require_approval_for: parseCSV(e.target.value),
                              },
                            })
                          }
                          placeholder={t(
                            "identity.agentRoles.detail.requireApprovalForPlaceholder",
                            "execution.run_command, connector.invoke_capability",
                          )}
                        />
                      </div>
                    </div>

                    {/* Model Binding */}
                    <div className="space-y-3">
                      <h3 className="text-sm font-bold text-muted-foreground uppercase tracking-widest">
                        {t("identity.agentRoles.detail.modelBinding")}
                      </h3>
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <div>
                          <label className="text-xs font-bold text-muted-foreground mb-1 block">
                            {t(
                              "identity.agentRoles.detail.modelBindingProvider",
                              "Primary Provider",
                            )}
                          </label>
                          <Input
                            value={
                              roleModelBinding(formData).primary?.provider_id ||
                              ""
                            }
                            onChange={(e) =>
                              updateForm({
                                model_binding: {
                                  ...roleModelBinding(formData),
                                  primary: normalizeModelBindingTarget({
                                    ...roleModelBinding(formData).primary,
                                    provider_id: e.target.value,
                                  }),
                                },
                              })
                            }
                            placeholder="openai-main"
                          />
                        </div>
                        <div>
                          <label className="text-xs font-bold text-muted-foreground mb-1 block">
                            {t(
                              "identity.agentRoles.detail.modelBindingModel",
                              "Primary Model",
                            )}
                          </label>
                          <Input
                            value={
                              roleModelBinding(formData).primary?.model || ""
                            }
                            onChange={(e) =>
                              updateForm({
                                model_binding: {
                                  ...roleModelBinding(formData),
                                  primary: normalizeModelBindingTarget({
                                    ...roleModelBinding(formData).primary,
                                    model: e.target.value,
                                  }),
                                },
                              })
                            }
                            placeholder={t(
                              "identity.agentRoles.detail.modelBindingModelPlaceholder",
                              "gpt-4.1-mini",
                            )}
                          />
                        </div>
                        <div>
                          <label className="text-xs font-bold text-muted-foreground mb-1 block">
                            {t(
                              "identity.agentRoles.detail.modelBindingFallbackProvider",
                              "Fallback Provider",
                            )}
                          </label>
                          <Input
                            value={
                              roleModelBinding(formData).fallback
                                ?.provider_id || ""
                            }
                            onChange={(e) =>
                              updateForm({
                                model_binding: {
                                  ...roleModelBinding(formData),
                                  fallback: normalizeModelBindingTarget({
                                    ...roleModelBinding(formData).fallback,
                                    provider_id: e.target.value,
                                  }),
                                },
                              })
                            }
                            placeholder="openai-backup"
                          />
                        </div>
                        <div>
                          <label className="text-xs font-bold text-muted-foreground mb-1 block">
                            {t(
                              "identity.agentRoles.detail.modelBindingFallbackModel",
                              "Fallback Model",
                            )}
                          </label>
                          <Input
                            value={
                              roleModelBinding(formData).fallback?.model || ""
                            }
                            onChange={(e) =>
                              updateForm({
                                model_binding: {
                                  ...roleModelBinding(formData),
                                  fallback: normalizeModelBindingTarget({
                                    ...roleModelBinding(formData).fallback,
                                    model: e.target.value,
                                  }),
                                },
                              })
                            }
                            placeholder={t(
                              "identity.agentRoles.detail.modelBindingFallbackModelPlaceholder",
                              "gpt-4o-mini",
                            )}
                          />
                        </div>
                      </div>
                      <label className="flex items-center gap-2 text-sm text-foreground">
                        <input
                          type="checkbox"
                          className="size-4 rounded border-border text-primary focus:ring-primary/40"
                          checked={
                            roleModelBinding(formData)
                              .inherit_platform_default ?? true
                          }
                          onChange={(e) =>
                            updateForm({
                              model_binding: {
                                ...roleModelBinding(formData),
                                inherit_platform_default: e.target.checked,
                              },
                            })
                          }
                        />
                        <span>
                          {t(
                            "identity.agentRoles.detail.modelBindingInheritPlatformDefault",
                            "Inherit platform primary when no role-specific primary is configured",
                          )}
                        </span>
                      </label>
                      <div className="text-xs text-muted-foreground">
                        {t("identity.agentRoles.detail.modelBindingHint")}
                      </div>
                    </div>

                    <div className="flex gap-3 pt-4 border-t border-border">
                      <Button
                        variant="amber"
                        onClick={onSave}
                        disabled={
                          createMutation.isPending || updateMutation.isPending
                        }
                      >
                        <Save size={14} />{" "}
                        {isCreating
                          ? t("identity.agentRoles.detail.create")
                          : t("identity.agentRoles.detail.save")}
                      </Button>
                      <Button
                        variant="outline"
                        onClick={() => {
                          if (isCreating) {
                            setIsCreating(false);
                            setSelectedID("");
                          } else setPanelMode("detail");
                        }}
                      >
                        <X size={14} /> {t("identity.agentRoles.detail.cancel")}
                      </Button>
                    </div>
                  </div>
                ) : (
                  selectedRole && (
                    <div className="space-y-8 animate-fade-in">
                      {/* Profile Section */}
                      <div className="space-y-4">
                        <h3 className="text-sm font-bold text-muted-foreground uppercase tracking-widest flex items-center gap-2">
                          <Bot size={16} />{" "}
                          {t("identity.agentRoles.detail.profile")}
                        </h3>
                        <div className="space-y-2">
                          <div className="text-xs font-bold text-muted-foreground">
                            {t("identity.agentRoles.detail.systemPrompt")}
                          </div>
                          <pre className="whitespace-pre-wrap text-sm bg-muted border border-border rounded-xl p-4 max-h-40 overflow-auto font-sans leading-relaxed text-foreground">
                            {selectedRole.profile?.system_prompt ||
                              t("common.none")}
                          </pre>
                        </div>
                        <div className="flex flex-wrap gap-2">
                          {selectedRole.profile?.persona_tags?.map((tag) => (
                            <span
                              key={tag}
                              className="px-3 py-1 rounded-full bg-primary/10 text-primary border border-primary/20 text-xs font-bold"
                            >
                              {tag}
                            </span>
                          ))}
                        </div>
                      </div>

                      {/* Capability Binding */}
                      <div className="space-y-4">
                        <h3 className="text-sm font-bold text-muted-foreground uppercase tracking-widest flex items-center gap-2">
                          <Zap size={16} />{" "}
                          {t("identity.agentRoles.detail.capabilityBinding")}
                        </h3>
                        <div className="flex flex-wrap gap-3">
                          <span className="px-3 py-1 rounded-full border border-border bg-muted text-xs font-bold text-foreground capitalize">
                            {t("identity.agentRoles.detail.modeLabel", {
                              mode:
                                selectedRole.capability_binding?.mode ||
                                t("common.na"),
                            })}
                          </span>
                        </div>
                        {(selectedRole.capability_binding?.allowed_skills
                          ?.length ?? 0) > 0 && (
                          <div>
                            <div className="text-xs font-bold text-muted-foreground mb-1 block uppercase tracking-wider">
                              {t("identity.agentRoles.detail.allowedSkillsCSV")}
                            </div>
                            <div className="flex flex-wrap gap-2">
                              {selectedRole.capability_binding?.allowed_skills?.map(
                                (s) => (
                                  <span
                                    key={s}
                                    className="px-2 py-0.5 rounded bg-muted text-xs font-mono border border-border text-foreground"
                                  >
                                    {s}
                                  </span>
                                ),
                              )}
                            </div>
                          </div>
                        )}
                        {(selectedRole.capability_binding?.allowed_skill_tags
                          ?.length ?? 0) > 0 && (
                          <div>
                            <div className="text-xs font-bold text-muted-foreground mb-1 block uppercase tracking-wider">
                              {t(
                                "identity.agentRoles.detail.allowedSkillTags",
                                "Allowed Skill Tags",
                              )}
                            </div>
                            <div className="flex flex-wrap gap-2">
                              {selectedRole.capability_binding?.allowed_skill_tags?.map(
                                (tag) => (
                                  <span
                                    key={tag}
                                    className="px-2 py-0.5 rounded bg-primary/10 text-primary text-xs font-mono border border-primary/20"
                                  >
                                    {tag}
                                  </span>
                                ),
                              )}
                            </div>
                          </div>
                        )}
                        {(selectedRole.capability_binding
                          ?.allowed_connector_capabilities?.length ?? 0) >
                          0 && (
                          <div>
                            <div className="text-xs font-bold text-muted-foreground mb-1 block uppercase tracking-wider">
                              {t(
                                "identity.agentRoles.detail.allowedConnectorCapabilities",
                                "Allowed Connector Capabilities",
                              )}
                            </div>
                            <div className="flex flex-wrap gap-2">
                              {selectedRole.capability_binding?.allowed_connector_capabilities?.map(
                                (capability) => (
                                  <span
                                    key={capability}
                                    className="px-2 py-0.5 rounded bg-muted text-xs font-mono border border-border text-foreground"
                                  >
                                    {capability}
                                  </span>
                                ),
                              )}
                            </div>
                          </div>
                        )}
                        {(selectedRole.capability_binding
                          ?.denied_connector_capabilities?.length ?? 0) > 0 && (
                          <div>
                            <div className="text-xs font-bold text-muted-foreground mb-1 block uppercase tracking-wider">
                              {t(
                                "identity.agentRoles.detail.deniedConnectorCapabilities",
                                "Denied Connector Capabilities",
                              )}
                            </div>
                            <div className="flex flex-wrap gap-2">
                              {selectedRole.capability_binding?.denied_connector_capabilities?.map(
                                (capability) => (
                                  <span
                                    key={capability}
                                    className="px-2 py-0.5 rounded bg-destructive/10 text-destructive text-xs font-mono border border-destructive/20"
                                  >
                                    {capability}
                                  </span>
                                ),
                              )}
                            </div>
                          </div>
                        )}
                      </div>

                      {/* Policy Binding */}
                      <div className="space-y-4">
                        <h3 className="text-sm font-bold text-muted-foreground uppercase tracking-widest flex items-center gap-2">
                          <Shield size={16} />{" "}
                          {t("identity.agentRoles.detail.policyBinding")}
                        </h3>
                        <div className="flex flex-wrap gap-3">
                          <span
                            className={clsx(
                              "px-3 py-1 rounded-full border text-xs font-bold uppercase",
                              selectedRole.policy_binding?.max_risk_level ===
                                "critical"
                                ? "bg-destructive/10 text-destructive border-destructive/20"
                                : selectedRole.policy_binding
                                      ?.max_risk_level === "warning"
                                  ? "bg-amber-500/10 text-amber-600 border-amber-500/20"
                                  : "bg-blue-500/10 text-blue-600 border-blue-500/20",
                            )}
                          >
                            {t("identity.agentRoles.detail.maxRiskLabel", {
                              risk:
                                selectedRole.policy_binding?.max_risk_level ||
                                t("common.na"),
                            })}
                          </span>
                          <span className="px-3 py-1 rounded-full border border-border bg-muted text-xs font-bold text-foreground uppercase tracking-tight">
                            {t("identity.agentRoles.detail.maxActionLabel", {
                              action:
                                selectedRole.policy_binding?.max_action?.replace(
                                  /_/g,
                                  " ",
                                ) || t("common.na"),
                            })}
                          </span>
                        </div>
                        {(selectedRole.policy_binding?.hard_deny?.length ?? 0) >
                          0 && (
                          <div>
                            <div className="text-xs font-bold text-muted-foreground mb-1 block uppercase tracking-wider">
                              {t("identity.agentRoles.detail.hardDeny")}
                            </div>
                            <div className="flex flex-wrap gap-2">
                              {selectedRole.policy_binding?.hard_deny?.map(
                                (s) => (
                                  <span
                                    key={s}
                                    className="px-2 py-0.5 rounded bg-destructive/10 text-destructive text-xs font-mono border border-destructive/20"
                                  >
                                    {s}
                                  </span>
                                ),
                              )}
                            </div>
                          </div>
                        )}
                        {(selectedRole.policy_binding?.require_approval_for
                          ?.length ?? 0) > 0 && (
                          <div>
                            <div className="text-xs font-bold text-muted-foreground mb-1 block uppercase tracking-wider">
                              {t(
                                "identity.agentRoles.detail.requireApprovalFor",
                                "Require Approval For",
                              )}
                            </div>
                            <div className="flex flex-wrap gap-2">
                              {selectedRole.policy_binding?.require_approval_for?.map(
                                (item) => (
                                  <span
                                    key={item}
                                    className="px-2 py-0.5 rounded bg-amber-500/10 text-amber-600 text-xs font-mono border border-amber-500/20"
                                  >
                                    {item}
                                  </span>
                                ),
                              )}
                            </div>
                          </div>
                        )}
                      </div>

                      {/* Model Binding */}
                      <div className="space-y-4">
                        <h3 className="text-sm font-bold text-muted-foreground uppercase tracking-widest flex items-center gap-2">
                          <Zap size={16} />{" "}
                          {t("identity.agentRoles.detail.modelBinding")}
                        </h3>
                        <div className="flex flex-wrap gap-2">
                          <span className="px-3 py-1 rounded-full border border-border bg-muted text-xs font-bold text-foreground">
                            {t(
                              "identity.agentRoles.detail.modelBindingProviderBadge",
                              {
                                provider:
                                  roleModelBinding(selectedRole).primary
                                    ?.provider_id || "auto",
                              },
                            )}
                          </span>
                          {roleModelBinding(selectedRole).primary?.model && (
                            <span className="px-3 py-1 rounded-full border border-border bg-muted text-xs font-bold text-foreground">
                              {t(
                                "identity.agentRoles.detail.modelBindingModelBadge",
                                {
                                  model:
                                    roleModelBinding(selectedRole).primary
                                      ?.model || t("common.na"),
                                },
                              )}
                            </span>
                          )}
                          {roleModelBinding(selectedRole).fallback
                            ?.provider_id && (
                            <span className="px-3 py-1 rounded-full border border-border bg-muted text-xs font-bold text-foreground">
                              {t(
                                "identity.agentRoles.detail.fallbackProviderBadge",
                                {
                                  provider: roleModelBinding(selectedRole).fallback?.provider_id || t("common.na"),
                                },
                              )}
                            </span>
                          )}
                          {roleModelBinding(selectedRole).fallback?.model && (
                            <span className="px-3 py-1 rounded-full border border-border bg-muted text-xs font-bold text-foreground">
                              {t(
                                "identity.agentRoles.detail.fallbackModelBadge",
                                {
                                  model: roleModelBinding(selectedRole).fallback?.model || t("common.na"),
                                },
                              )}
                            </span>
                          )}
                          <span className="px-3 py-1 rounded-full border border-border bg-muted text-xs font-bold text-foreground">
                            {t(
                              "identity.agentRoles.detail.inheritPlatformDefaultBadge",
                              {
                                value: roleModelBinding(selectedRole).inherit_platform_default ? t("common.yes") : t("common.no"),
                              },
                            )}
                          </span>
                        </div>
                        <div className="text-xs text-muted-foreground">
                          {t("identity.agentRoles.detail.modelBindingHint")}
                        </div>
                      </div>

                      {/* Metadata */}
                      <div className="space-y-2 text-xs text-muted-foreground border-t border-border pt-4">
                        {selectedRole.is_builtin && (
                          <div className="italic text-amber-600 font-medium">
                            {t("identity.agentRoles.detail.builtinNotice")}
                          </div>
                        )}
                        <div className="grid grid-cols-2 gap-4">
                          <div>
                            {t("identity.agentRoles.detail.createdAt", {
                              time: selectedRole.created_at
                                ? new Date(
                                    selectedRole.created_at,
                                  ).toLocaleString()
                                : t("common.na"),
                            })}
                          </div>
                          <div>
                            {t("identity.agentRoles.detail.updatedAt", {
                              time: selectedRole.updated_at
                                ? new Date(
                                    selectedRole.updated_at,
                                  ).toLocaleString()
                                : t("common.na"),
                            })}
                          </div>
                        </div>
                      </div>
                    </div>
                  )
                )}
              </div>
            </RegistryDetail>
          ) : (
            <EmptyDetailState
              title={t("identity.agentRoles.empty.title")}
              description={t("identity.agentRoles.empty.desc")}
            />
          )
        }
      />
    </div>
  );
};

export default AgentRolesPage;
