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
type IconName = "mail" | "user" | "shield" | "check" | "clock" | "cake" | "file" | "pencil" | "camera";

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
  const [avatarFailed, setAvatarFailed] = useState(false);

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
  const showAvatarImage = Boolean(avatarURL && !avatarFailed);
  const roleInfo = roleCapability(user?.role);

  useEffect(() => {
    setAvatarFailed(false);
  }, [avatarURL]);

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
      <Alert type="success" message={success} autoDismissMs={3000} onDismiss={() => setSuccess("")} />

      {loading ? (
        <div className="panel empty-state">正在加载当前用户信息...</div>
      ) : (
        <div className="profile-grid">
          <section className="panel profile-card">
            <div className="profile-hero">
              <div className="profile-hero-avatar">
                {/* 头像上传入口和头像强关联，用户能自然理解“相机按钮”只影响当前头像，不会误以为在编辑邮箱或角色。 */}
                {showAvatarImage ? (
                  <img src={avatarURL} alt="用户头像" onError={() => setAvatarFailed(true)} />
                ) : (
                  <div className="avatar-placeholder">{avatarInitial(user?.nickname, user?.email)}</div>
                )}
                <label className="avatar-camera-button" title="更换头像" aria-label="更换头像">
                  <Icon name="camera" />
                  <input type="file" accept=".jpg,.jpeg,.png,.webp,image/jpeg,image/png,image/webp" onChange={(event) => void onAvatarChange(event)} disabled={uploading} />
                </label>
              </div>
              <div className="profile-hero-main">
                <h3>{user?.nickname ?? "未命名用户"}</h3>
                <p>{user?.email}</p>
                <div className="profile-tag-row">
                  <span className="profile-role-pill"><Icon name="shield" />{roleLabel(user?.role)}</span>
                  <span className="profile-status-pill"><Icon name="check" />{statusLabel(user?.status)}</span>
                </div>
                {uploading && <span className="avatar-uploading">头像上传中...</span>}
              </div>
            </div>
            <div className="profile-summary-grid">
              <SummaryItem icon="check" label="账号状态" value={statusLabel(user?.status)} />
              <SummaryItem icon="clock" label="创建时间" value={formatDate(user?.created_at)} />
              <SummaryItem icon="cake" label="生日" value={user?.birthday || "未填写"} />
            </div>
            {/* 资料详情保留完整字段，但把身份卡已强调的信息降级展示，减少重复堆砌造成的阅读噪音。 */}
            <dl className="profile-details profile-details-card">
              <Detail icon="mail" label="邮箱" value={user?.email} />
              <Detail icon="user" label="昵称" value={user?.nickname} />
              <Detail icon="shield" label="角色" value={roleLabel(user?.role)} />
              <Detail icon="file" label="简介" value={user?.bio || "未填写"} />
            </dl>
            <div className="profile-role-card">
              <div className="profile-role-icon"><Icon name="shield" /></div>
              <div>
                <strong>{roleInfo.title}</strong>
                <p>{roleInfo.description}</p>
              </div>
            </div>
          </section>

          <form className="panel edit-card" onSubmit={(event) => void onSave(event)}>
            <div className="panel-header">
              <h3><Icon name="pencil" />编辑资料</h3>
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

function Detail({ icon, label, value }: { icon: IconName; label: string; value?: string | number | null }) {
  return (
    <div>
      <dt><Icon name={icon} />{label}</dt>
      <dd>{value || "-"}</dd>
    </div>
  );
}

function SummaryItem({ icon, label, value }: { icon: IconName; label: string; value?: string | number | null }) {
  return (
    <div className="summary-item">
      <span><Icon name={icon} />{label}</span>
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

function roleCapability(role?: string) {
  if (role === "data_owner") {
    return {
      title: "当前身份：数据拥有者",
      description: "可以上传文件、设置访问策略，并加密共享数据。"
    };
  }
  if (role === "data_user") {
    return {
      title: "当前身份：数据访问者",
      description: "可以查看授权文件、申请密钥，并在属性满足策略时解密访问数据。"
    };
  }
  if (role === "admin") {
    return {
      title: "当前身份：系统管理员",
      description: "后续可管理用户、系统配置和算法配置。"
    };
  }
  return {
    title: "当前身份：未识别角色",
    description: "请刷新资料或联系管理员确认账号角色。"
  };
}

function Icon({ name }: { name: IconName }) {
  const paths: Record<IconName, React.ReactNode> = {
    mail: <><path d="M4 6h16v12H4z" /><path d="m4 7 8 6 8-6" /></>,
    user: <><circle cx="12" cy="8" r="4" /><path d="M4 20c1.8-4 5-6 8-6s6.2 2 8 6" /></>,
    shield: <path d="M12 3 20 6v6c0 5-3.3 8-8 9-4.7-1-8-4-8-9V6l8-3Z" />,
    check: <><circle cx="12" cy="12" r="9" /><path d="m8.5 12.5 2.3 2.3 4.7-5.1" /></>,
    clock: <><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></>,
    cake: <><path d="M5 12h14v8H5z" /><path d="M8 12V9m4 3V9m4 3V9" /><path d="M7 16c1 1 2 1 3 0s2-1 3 0 2 1 4 0" /></>,
    file: <><path d="M6 3h8l4 4v14H6z" /><path d="M14 3v5h5M9 13h6M9 17h6" /></>,
    pencil: <><path d="M4 20h4l10-10-4-4L4 16v4Z" /><path d="m13 7 4 4" /></>,
    camera: <><path d="M4 8h4l1.5-2h5L16 8h4v11H4z" /><circle cx="12" cy="13.5" r="3" /></>
  };

  // 统一线性图标比大量 emoji 更专业，也能通过 currentColor 自动跟随主题色。
  return (
    <svg className="ui-icon" viewBox="0 0 24 24" aria-hidden="true">
      {paths[name]}
    </svg>
  );
}
