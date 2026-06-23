"use client";
import { registerServiceWorker } from "@/lib/push";
import { useEffect } from "react";

export default function ServiceWorkerRegistrar() {
  useEffect(() => {
    registerServiceWorker();
  }, []);
  return null;
}
