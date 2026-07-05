import { ChangeEvent, FormEvent, useEffect, useState } from "react";
import { getCurrentUser, updateCurrentUser, uploadAvatar } from "../api/user";
import { ApiError } from "../api/request";
import { useAuth } from "../auth/AuthContext";
import { Alert } from "../components/Alert";
import type { User } from "../types";
import { avatarInitial, resolveAvatarURL } from "../utils/avatar";

const MAX_AVATAR_SIZE = 2 * 1024 * 1024;
const ALLOWED_TYPES = ["image/jpeg", "image/png", "image/webp"];
const ALLOWED_EXTS = [".jpg", ".jpeg", ".png", ".webp"];

export function ProfilePage() {
  const auth = useAuth();
  const [user, setUser] = useState<User | null>(auth.user);
  const [nickname, setNickname] = useState(auth.user?.nickname ?? "");
  const [bio, setBio] = useState(auth.user?.bio ?? "");
  const [birthday, setBirthday] = useState(auth.user?.birthday ?? "");
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [uploading, setUploading] = useState(false);

  async function loadProfile() {
    setLoading(true);
    setError("");
    try {
      const data = await getCurrentUser();
      setProfile(data.user);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "获取当前用户失败");
    } finally {
      setLoading(false);
    }
  }

  function setProfile(nextUser: User) {
    setUser(nextUser);
    auth.setUser(nextUser);
    setNickname(nextUser.nickname ?? "");
    setBio(nextUser.bio ?? "");
    setBirthday(nextUser.birthday ?? "");
  }

  useEffect(() => {
    setUser(auth.user);
    setNickname(auth.user?.nickname ?? "");
    setBio(auth.user?.bio ?? "");
    setBirthday(auth.user?.birthday ?? "");
    void loadProfile();
  }, [auth.currentUserId]);

  async function onSave(event: FormEvent) {
    event.preventDefault();
    setError("");
    setSuccess("");
    if (!nickname.trim() || nickname.trim().length > 20) {
      setError("昵称长度必须为 1 到 20 个字符");
      return;
    }
    if (bio.length > 200) {
      setError("个人简介不能超过 200 字");
      return;
    }
    if (birthday && !/^\d{4}-\d{2}-\d{2}$/.test(birthday)) {
      setError("生日格式必须为 YYYY-MM-DD");
      return;
    }
    setSaving(true);
    try {
      const data = await updateCurrentUser({ nickname: nickname.trim(), bio, birthday });
      setProfile(data.user);
      setSuccess("资料已保存");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "编辑资料失败");
    } finally {
      setSaving(false);
    }
  }

  async function onAvatarChange(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    event.target.value = "";
    setError("");
    setSuccess("");
    if (!file) return;
    const ext = file.name.slice(file.name.lastIndexOf(".")).toLowerCase();
    if (!ALLOWED_TYPES.includes(file.type) && !ALLOWED_EXTS.includes(ext)) {
      setError("只允许上传 jpg、jpeg、png、webp 图片");
      return;
    }
    if (file.size > MAX_AVATAR_SIZE) {
      setError("头像文件不能超过 2MB");
      return;
    }
    setUploading(true);
    try {
      await uploadAvatar(file);
      const data = await getCurrentUser();
      // 上传接口只返回头像字段时，重新拉取完整用户并写入全局状态，确保侧边栏头像和资料页同时更新。
      setProfile(data.user);
      setSuccess("头像已更新");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "上传头像失败");
    } finally {
      setUploading(false);
    }
  }

  const avatarURL = resolveAvatarURL(user?.avatar_url ?? "");

  return (
    <section className="profile-page">
      <div className="profile-header">
        <div>
          <h2>个人中心</h2>
          <p>维护账号资料，确认当前角色和加密共享系统中的身份信息</p>
        </div>
        <button className="secondary-action" type="button" onClick={() => void loadProfile()} disabled={loading}>
          刷新资料
        </button>
      </div>

      <Alert type="error" message={error} />
      <Alert type="success" message={success} />

      {loading ? (
        <div className="panel empty-state">正在加载当前用户信息...</div>
      ) : (
        <div className="profile-grid">
          <section className="panel profile-card">
            <div className="profile-hero">
              <div className="profile-hero-avatar">
                {avatarURL ? <img src={avatarURL} alt="用户头像" /> : <div className="avatar-placeholder">{avatarInitial(user?.nickname, user?.email)}</div>}
              </div>
              <div className="profile-hero-main">
                <span className="profile-role-pill">{roleLabel(user?.role)}</span>
                <h3>{user?.nickname ?? "未命名用户"}</h3>
                <p>{user?.email}</p>
                <label className="upload-button">
                  {uploading ? "上传中..." : "更换头像"}
                  <input type="file" accept=".jpg,.jpeg,.png,.webp,image/jpeg,image/png,image/webp" onChange={(event) => void onAvatarChange(event)} disabled={uploading} />
                </label>
              </div>
            </div>
            <div className="profile-summary-grid">
              <SummaryItem label="账号状态" value={statusLabel(user?.status)} />
              <SummaryItem label="创建时间" value={formatDate(user?.created_at)} />
              <SummaryItem label="生日" value={user?.birthday || "未填写"} />
            </div>
            <dl className="profile-details profile-details-card">
              <Detail label="邮箱" value={user?.email} />
              <Detail label="昵称" value={user?.nickname} />
              <Detail label="角色" value={roleLabel(user?.role)} />
              <Detail label="简介" value={user?.bio || "未填写"} />
            </dl>
          </section>

          <form className="panel edit-card" onSubmit={(event) => void onSave(event)}>
            <div className="panel-header">
              <h3>编辑资料</h3>
            </div>
            <label className="field">
              <span>昵称</span>
              <input value={nickname} maxLength={20} onChange={(event) => setNickname(event.target.value)} />
            </label>
            <label className="field">
              <span>个人简介</span>
              <textarea value={bio} maxLength={200} onChange={(event) => setBio(event.target.value)} />
            </label>
            <label className="field">
              <span>生日</span>
              <input value={birthday ?? ""} type="date" onChange={(event) => setBirthday(event.target.value)} />
            </label>
            <button className="primary-action" type="submit" disabled={saving}>
              {saving ? "保存中..." : "保存资料"}
            </button>
          </form>
        </div>
      )}
    </section>
  );
}

function Detail({ label, value }: { label: string; value?: string | number | null }) {
  return (
    <div>
      <dt>{label}</dt>
      <dd>{value || "-"}</dd>
    </div>
  );
}

function SummaryItem({ label, value }: { label: string; value?: string | number | null }) {
  return (
    <div className="summary-item">
      <span>{label}</span>
      <strong>{value || "-"}</strong>
    </div>
  );
}

function roleLabel(role?: string) {
  if (role === "data_owner") return "数据拥有者";
  if (role === "data_user") return "数据访问者";
  if (role === "admin") return "系统管理员";
  return "-";
}

function statusLabel(status?: string) {
  if (status === "active") return "启用";
  if (status === "disabled") return "禁用";
  return "-";
}

function formatDate(value?: string) {
  if (!value) return "-";
  return new Date(value).toLocaleString();
}
