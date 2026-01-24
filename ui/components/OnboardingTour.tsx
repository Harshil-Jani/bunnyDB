'use client';

import { useEffect, useMemo } from 'react';
import { NextStepProvider, NextStep, useNextStep } from 'nextstepjs';
import { getTourSteps } from '../lib/tour-steps';
import { TourCard } from './TourCard';
import { getToken, isAdmin } from '../lib/auth';

function TourTrigger() {
  const { startNextStep } = useNextStep();

  useEffect(() => {
    const token = getToken();
    if (!token) return;

    const hasSeenTour = localStorage.getItem('bunny_tour_seen');
    if (!hasSeenTour) {
      const timer = setTimeout(() => {
        startNextStep('onboarding');
        localStorage.setItem('bunny_tour_seen', '1');
      }, 800);
      return () => clearTimeout(timer);
    }
  }, [startNextStep]);

  return null;
}

export function OnboardingTour({ children }: { children: React.ReactNode }) {
  const role = isAdmin() ? 'admin' : 'readonly';
  const steps = useMemo(() => getTourSteps(role), [role]);

  return (
    <NextStepProvider>
      <NextStep
        steps={steps}
        cardComponent={TourCard}
        shadowRgb="0, 0, 0"
        shadowOpacity="0.5"
        onComplete={() => {
          localStorage.setItem('bunny_tour_seen', '1');
        }}
        onSkip={() => {
          localStorage.setItem('bunny_tour_seen', '1');
        }}
      >
        <TourTrigger />
        {children}
      </NextStep>
    </NextStepProvider>
  );
}
