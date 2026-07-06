import type { CSSProperties } from "react";

export type LoginRobotState = "normal" | "email-focus" | "password-focus" | "password-visible" | "error" | "success";

interface LoginRobotProps {
  state: LoginRobotState;
  eyeOffset: { x: number; y: number };
}

const stateText: Record<LoginRobotState, string> = {
  normal: "安全助手待命，准备校验登录凭证",
  "email-focus": "正在观察邮箱输入",
  "password-focus": "密码输入中，安全助手遮挡视线",
  "password-visible": "密码可见，安全助手半遮视线",
  error: "登录失败，请重新确认凭证",
  success: "身份校验通过，正在进入工作台"
};

// 机器人拆成独立组件，避免登录表单里混入大量视觉结构，也方便后续复用到注册或安全验证场景。
export function LoginRobot({ state, eyeOffset }: LoginRobotProps) {
  const eyeStyle = {
    "--eye-x": `${eyeOffset.x}px`,
    "--eye-y": `${eyeOffset.y}px`
  } as CSSProperties;

  return (
    <div className={`login-robot login-robot-${state}`} aria-label={stateText[state]} role="img" style={eyeStyle}>
      <div className="login-robot-halo" />
      <div className="login-robot-antenna login-robot-antenna-left" />
      <div className="login-robot-antenna login-robot-antenna-right" />
      <div className="login-robot-ear login-robot-ear-left" />
      <div className="login-robot-ear login-robot-ear-right" />
      <div className="login-robot-head">
        <div className="login-robot-face">
          <span className="login-robot-eye login-robot-eye-left" />
          <span className="login-robot-eye login-robot-eye-right" />
          <span className="login-robot-mouth" />
        </div>
        <span className="login-robot-gloss" />
      </div>
      <div className="login-robot-body">
        <span className="login-robot-core" />
      </div>
      <div className="login-robot-arm login-robot-arm-left">
        <span className="login-robot-hand" />
      </div>
      <div className="login-robot-arm login-robot-arm-right">
        <span className="login-robot-hand" />
      </div>
      <span className="login-robot-badge" />
      <p>{stateText[state]}</p>
    </div>
  );
}
