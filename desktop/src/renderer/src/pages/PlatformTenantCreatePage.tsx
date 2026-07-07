import { FormEvent, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { createPlatformTenant } from "../api/platform";
import { ApiError } from "../api/request";
import { Alert } from "../components/Alert";

const TENANT_CODE_PATTERN = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

export function PlatformTenantCreatePage() {
  const navigate = useNavigate();
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [description, setDescription] = useState("");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    setError("");
    const normalizedName = name.trim();
    const normalizedCode = code.trim().toLowerCase();
    if (!normalizedName) {
      setError("请输入租户名称");
      return;
    }
    if (!normalizedCode) {
      setError("请输入租户编码");
      return;
    }
    if (!TENANT_CODE_PATTERN.test(normalizedCode)) {
      setError("租户编码只能包含小写字母、数字和中划线，且不能以中划线开头或结尾");
      return;
    }
    setSaving(true);
    try {
      const tenant = await createPlatformTenant({ name: normalizedName, code: normalizedCode, description: description.trim() });
      navigate(`/platform/tenants/${tenant.tenant_id}`, { replace: true, state: { notice: "租户已创建" } });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "创建租户失败");
    } finally {
      setSaving(false);
    }
  }

  return (
    <section className="platform-page">
      <div className="platform-header">
        <div>
          <h2>创建租户</h2>
          <p>为新的组织边界创建租户，编码创建后将作为登录入口标识</p>
        </div>
        <Link className="secondary-action" to="/platform/tenants">
          返回列表
        </Link>
      </div>

      <Alert type="error" message={error} />

      <form className="panel platform-form" onSubmit={(event) => void onSubmit(event)}>
        <div className="platform-form-grid">
          <label className="field">
            <span>租户名称</span>
            <input value={name} maxLength={128} onChange={(event) => setName(event.target.value)} placeholder="例如：深信服科技" />
          </label>
          <label className="field">
            <span>租户编码</span>
            <input
              value={code}
              maxLength={64}
              onChange={(event) => setCode(event.target.value.toLowerCase())}
              placeholder="例如：sangfor"
            />
          </label>
        </div>
        <label className="field">
          <span>租户描述</span>
          <textarea value={description} maxLength={512} onChange={(event) => setDescription(event.target.value)} placeholder="用于说明租户用途，不填写也可以" />
        </label>
        <div className="platform-actions platform-actions-left">
          <button className="primary-action" type="submit" disabled={saving}>
            {saving ? "创建中..." : "创建租户"}
          </button>
          <Link className="secondary-action" to="/platform/tenants">
            取消
          </Link>
        </div>
      </form>
    </section>
  );
}
