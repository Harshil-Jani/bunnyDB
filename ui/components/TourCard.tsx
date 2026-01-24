'use client';

import type { CardComponentProps } from 'nextstepjs';

export function TourCard({
  step,
  currentStep,
  totalSteps,
  nextStep,
  prevStep,
  skipTour,
  arrow,
}: CardComponentProps) {
  const isLast = currentStep === totalSteps - 1;
  const isFirst = currentStep === 0;

  return (
    <div className="relative bg-white dark:bg-gray-900 rounded-lg shadow-lg dark:shadow-gray-900/50 border border-gray-200 dark:border-gray-700 p-3.5 w-[260px]">
      {arrow}

      {/* Header */}
      <div className="flex items-center justify-between mb-1.5">
        <div className="flex items-center gap-1.5">
          {step.icon && <span className="text-sm">{step.icon}</span>}
          <h3 className="text-xs font-semibold text-gray-900 dark:text-white">
            {step.title}
          </h3>
        </div>
        <span className="text-[10px] text-gray-400 dark:text-gray-500 tabular-nums">
          {currentStep + 1}/{totalSteps}
        </span>
      </div>

      {/* Content */}
      <div className="text-[11px] text-gray-500 dark:text-gray-400 leading-relaxed mb-3">
        {step.content}
      </div>

      {/* Progress + Controls */}
      <div className="flex items-center gap-2">
        <div className="flex-1 h-0.5 bg-gray-100 dark:bg-gray-800 rounded-full overflow-hidden">
          <div
            className="h-full bg-bunny-500 rounded-full transition-all duration-300"
            style={{ width: `${((currentStep + 1) / totalSteps) * 100}%` }}
          />
        </div>
        <div className="flex items-center gap-1">
          {step.showSkip && (
            <button
              onClick={skipTour}
              className="text-[10px] text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 px-1.5 py-0.5"
            >
              Skip
            </button>
          )}
          {!isFirst && (
            <button
              onClick={prevStep}
              className="text-[10px] font-medium text-gray-500 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white px-1.5 py-0.5"
            >
              Back
            </button>
          )}
          <button
            onClick={nextStep}
            className="px-2.5 py-1 text-[10px] font-medium bg-gray-900 dark:bg-white text-white dark:text-gray-900 rounded-md hover:bg-gray-800 dark:hover:bg-gray-100"
          >
            {isLast ? 'Done' : 'Next'}
          </button>
        </div>
      </div>
    </div>
  );
}
