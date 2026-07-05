import { createApp } from "./App";
import "./styles.css";

const root = document.querySelector<HTMLDivElement>("#app");

if (!root) {
  throw new Error("renderer root element not found");
}

root.appendChild(createApp());
