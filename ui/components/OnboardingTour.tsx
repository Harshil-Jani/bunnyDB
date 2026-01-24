'use client';

import { useEffect } from 'react';
import { NextStepProvider, NextStep, useNextStep } from 'nextstepjs';
import { tourSteps } from '../lib/tour-steps';
import { TourCard } from './TourCard';
import { getToken } from '../lib/auth';

function TourTrigger() {
  const { startNextStep } = useNextStep();

  useEffect(() => {
    const token = getToken();
    if (!token) return;

    const hasSeenTour = localStorage.getItem('bunny_tour_seen');
    if (!hasSeenTour) {
      // Small delay to let the page render and IDs to be available
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
  return (
    <NextStepProvider>
      <NextStep
        steps={tourSteps}
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
