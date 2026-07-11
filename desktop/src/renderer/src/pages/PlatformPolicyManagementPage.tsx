import { useEffect, useState } from "react";
import { createPlatformAttribute, createPlatformTemplate, deletePlatformAttribute, deletePlatformTemplate, listPlatformAttributes, listPlatformTemplates } from "../api/policy";
import { Alert } from "../components/Alert";
import { mockTree } from "../components/access-policy/tree/mockData";
import { attributeCode, attributeName, attributeType, type PolicyAttribute, type PolicyTemplate } from "../components/access-policy/tree/types";

export function PlatformPolicyManagementPage() {
  const [attributes, setAttributes] = useState<PolicyAttribute[]>([]);
  const [templates, setTemplates] = useState<PolicyTemplate[]>([]);
  const [message, setMessage] = useState("");
  const [loading, setLoading] = useState(true);
  const [workingKey, setWorkingKey] = useState("");

  async function load() {
    setLoading(true);
    setMessage("");
    try {
      const [nextAttrs, nextTemplates] = await Promise.all([listPlatformAttributes(), listPlatformTemplates()]);
      setAttributes(nextAttrs);
      setTemplates(nextTemplates);
    } catch {
      setMessage("加载访问策略配置失败");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { void load(); }, []);

  async function seedAttribute() {
    setWorkingKey("seed-attribute");
    try {
      await createPlatformAttribute({ attrCode: "department", attrName: "部门", attrType: "enum", attrValues: ["研发部", "财务部"], status: "enabled" });
      await load();
    } catch {
      setMessage("创建属性失败");
    } finally {
      setWorkingKey("");
    }
  }

  async function seedTemplate() {
    setWorkingKey("seed-template");
    try {
      await createPlatformTemplate({ name: "数据拥有者或租户管理员可访问", description: "常用模板", policyTreeJson: mockTree, status: "enabled" });
      await load();
    } catch {
      setMessage("创建模板失败");
    } finally {
      setWorkingKey("");
    }
  }

  async function removeAttribute(attribute: PolicyAttribute) {
    if (!window.confirm(`确认删除属性 ${attributeName(attribute)}？`)) return;
    setWorkingKey(`attr-${attribute.id}`);
    try {
      await deletePlatformAttribute(attribute.id);
      await load();
    } catch {
      setMessage("删除属性失败");
    } finally {
      setWorkingKey("");
    }
  }

  async function removeTemplate(template: PolicyTemplate) {
    if (!window.confirm(`确认删除模板 ${template.name}？`)) return;
    setWorkingKey(`template-${template.id}`);
    try {
      await deletePlatformTemplate(template.id);
      await load();
    } catch {
      setMessage("删除模板失败");
    } finally {
      setWorkingKey("");
    }
  }

  return (
    <section className="platform-page">
      <div className="platform-header">
        <div>
          <h2>访问策略管理</h2>
          <p>维护平台级属性字典和策略模板，为租户策略构建提供基础配置</p>
        </div>
        <div className="platform-actions">
          <button className="secondary-action" type="button" onClick={() => void load()} disabled={loading}>刷新</button>
          <button className="primary-action" type="button" onClick={() => void seedAttribute()} disabled={Boolean(workingKey)}>添加示例属性</button>
          <button className="secondary-action" type="button" onClick={() => void seedTemplate()} disabled={Boolean(workingKey)}>添加示例模板</button>
        </div>
      </div>

      <Alert type="error" message={message} />

      <div className="policy-admin-grid">
        <section className="panel policy-admin-panel">
          <div className="panel-header">
            <div>
              <h3>属性字典</h3>
              <p>属性名、编码、类型与启用状态</p>
            </div>
          </div>
          <div className="platform-table-wrap">
            {loading ? (
              <div className="empty-state">正在加载属性字典...</div>
            ) : attributes.length > 0 ? (
              <table className="platform-table compact-table">
                <thead>
                  <tr>
                    <th>属性名</th>
                    <th>编码</th>
                    <th>类型</th>
                    <th>状态</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {attributes.map((attr) => (
                    <tr key={attr.id}>
                      <td><strong>{attributeName(attr)}</strong></td>
                      <td><code className="table-code">{attributeCode(attr)}</code></td>
                      <td><span className="status-badge status-muted">{attributeType(attr)}</span></td>
                      <td><StatusTag status={attr.status} /></td>
                      <td>
                        <button className="secondary-action danger-action" type="button" onClick={() => void removeAttribute(attr)} disabled={workingKey === `attr-${attr.id}`}>
                          {workingKey === `attr-${attr.id}` ? "删除中..." : "删除"}
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : (
              <div className="empty-state">暂无属性字典。</div>
            )}
          </div>
        </section>

        <section className="panel policy-admin-panel">
          <div className="panel-header">
            <div>
              <h3>策略模板</h3>
              <p>平台预置的常用策略表达</p>
            </div>
          </div>
          <div className="platform-table-wrap">
            {loading ? (
              <div className="empty-state">正在加载策略模板...</div>
            ) : templates.length > 0 ? (
              <div className="policy-template-list">
                {templates.map((template) => (
                  <article className="policy-template-item" key={template.id}>
                    <div className="policy-template-main">
                      <div className="policy-template-title-row">
                        <strong title={template.name}>{template.name}</strong>
                        <StatusTag status={template.status} />
                      </div>
                      <p title={template.description || "暂无说明"}>{template.description || "暂无说明"}</p>
                    </div>
                    <div className="policy-template-actions">
                      <button className="secondary-action danger-action" type="button" onClick={() => void removeTemplate(template)} disabled={workingKey === `template-${template.id}`}>
                        {workingKey === `template-${template.id}` ? "删除中..." : "删除"}
                      </button>
                    </div>
                  </article>
                ))}
              </div>
            ) : (
              <div className="empty-state">暂无策略模板。</div>
            )}
          </div>
        </section>
      </div>
    </section>
  );
}

function StatusTag({ status }: { status?: string }) {
  return <span className={`status-badge ${status === "enabled" ? "status-enabled" : "status-disabled"}`}>{status === "enabled" ? "启用" : "禁用"}</span>;
}
