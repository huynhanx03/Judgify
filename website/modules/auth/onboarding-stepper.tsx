"use client";

import { useEffect, useRef } from "react";
import { useLocale } from "@/contexts/locale-context";

export interface OnboardingStep {
  id: string;
  label: string;
}

export function OnboardingStepper({
  steps,
  activeStep,
  title,
  children,
}: {
  steps: OnboardingStep[];
  activeStep: number;
  title: string;
  children: React.ReactNode;
}) {
  const { t } = useLocale();
  const headingRef = useRef<HTMLHeadingElement>(null);

  useEffect(() => {
    headingRef.current?.focus();
  }, [activeStep]);

  const step = steps[activeStep];
  if (!step) return null;

  return (
    <section aria-labelledby="onboarding-step-heading">
      <p className="mb-2 text-xs font-medium text-muted-foreground" aria-live="polite">
        {t("AUTH.ONBOARDING_STEP", { args: [activeStep + 1, steps.length, step.label] })}
      </p>
      <h2
        id="onboarding-step-heading"
        ref={headingRef}
        tabIndex={-1}
        className="mb-4 text-lg font-semibold outline-none"
      >
        {title}
      </h2>
      {children}
    </section>
  );
}
