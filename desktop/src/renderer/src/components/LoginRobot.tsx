import { useEffect, useState, type CSSProperties } from "react";

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

const assistantMessages = [
  "安全助手待命，准备校验登录凭证",
  "基于 CP-ABE 策略加密，守护你的共享文件",
  "只有满足访问策略的用户，才能完成解密访问",
  "支持访问树可视化，授权规则清晰可见",
  "密文优先存储，降低明文数据泄露风险",
  "正在为你准备安全的文件共享工作台"
];

// 机器人拆成独立组件，避免登录表单里混入大量视觉结构，也方便后续复用到注册或安全验证场景。
export function LoginRobot({ state, eyeOffset }: LoginRobotProps) {
  const [messageIndex, setMessageIndex] = useState(0);
  const [visibleLength, setVisibleLength] = useState(0);
  const [prefersReducedMotion, setPrefersReducedMotion] = useState(false);
  const eyeStyle = {
    "--eye-x": `${eyeOffset.x}px`,
    "--eye-y": `${eyeOffset.y}px`
  } as CSSProperties;
  const dynamicMessage = assistantMessages[messageIndex];
  const isAssistantIntro = state === "normal";
  const robotMessage = isAssistantIntro ? dynamicMessage.slice(0, visibleLength) : stateText[state];
  const ariaLabel = isAssistantIntro ? dynamicMessage : stateText[state];

  useEffect(() => {
    const mediaQuery = window.matchMedia("(prefers-reduced-motion: reduce)");
    setPrefersReducedMotion(mediaQuery.matches);

    function onChange(event: MediaQueryListEvent) {
      setPrefersReducedMotion(event.matches);
    }

    mediaQuery.addEventListener("change", onChange);
    return () => mediaQuery.removeEventListener("change", onChange);
  }, []);

  useEffect(() => {
    if (!isAssistantIntro) {
      setVisibleLength(stateText[state].length);
      return;
    }

    if (prefersReducedMotion) {
      setVisibleLength(dynamicMessage.length);
      const timer = window.setTimeout(() => {
        setMessageIndex((index) => (index + 1) % assistantMessages.length);
      }, 7000);
      return () => window.clearTimeout(timer);
    }

    if (visibleLength > dynamicMessage.length) {
      setVisibleLength(0);
      return;
    }

    if (visibleLength < dynamicMessage.length) {
      const timer = window.setTimeout(() => {
        setVisibleLength((length) => length + 1);
      }, 46);
      return () => window.clearTimeout(timer);
    }

    const timer = window.setTimeout(() => {
      setMessageIndex((index) => (index + 1) % assistantMessages.length);
      setVisibleLength(0);
    }, 3200);
    return () => window.clearTimeout(timer);
  }, [dynamicMessage, isAssistantIntro, prefersReducedMotion, state, visibleLength]);

  return (
    <div className={`login-robot login-robot-${state}`} aria-label={ariaLabel} role="img" style={eyeStyle}>
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
      <p className="login-robot-message">
        {robotMessage}
        {isAssistantIntro && !prefersReducedMotion && <span className="login-robot-cursor" aria-hidden="true" />}
      </p>
    </div>
  );
}
