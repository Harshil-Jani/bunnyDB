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
    <div className="relative bg-white dark:bg-gray-900 rounded-xl shadow-lg dark:shadow-gray-900/50 border border-gray-200 dark:border-gray-700 p-5 max-w-sm w-full">
      {arrow}

      {/* Header */}
      <div className="flex items-start justify-between mb-3">
        <div className="flex items-center gap-2">
          {step.icon && <span className="text-lg">{step.icon}</span>}
          <h3 className="text-base font-semibold text-gray-900 dark:text-white">
            {step.title}
          </h3>
        </div>
        <span className="text-[11px] text-gray-400 dark:text-gray-500 font-medium tabular-nums whitespace-nowrap ml-3">
          {currentStep + 1}/{totalSteps}
        </span>
      </div>

      {/* Content */}
      <div className="text-sm text-gray-600 dark:text-gray-300 leading-relaxed">
        {step.content}
      </div>

      {/* Progress bar */}
      <div className="mt-4 mb-3 h-1 bg-gray-100 dark:bg-gray-800 rounded-full overflow-hidden">
        <div
          className="h-full bg-bunny-500 rounded-full transition-all duration-300"
          style={{ width: `${((currentStep + 1) / totalSteps) * 100}%` }}
        />
      </div>

      {/* Controls */}
      <div className="flex items-center justify-between">
        <div>
          {step.showSkip && (
            <button
              onClick={skipTour}
              className="text-xs text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
            >
              Skip tour
            </button>
          )}
        </div>
        <div className="flex items-center gap-2">
          {!isFirst && (
            <button
              onClick={prevStep}
              className="px-3 py-1.5 text-xs font-medium text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white transition-colors"
            >
              Back
            </button>
          )}
          <button
            onClick={nextStep}
            className="px-4 py-1.5 text-xs font-medium bg-gray-900 dark:bg-white text-white dark:text-gray-900 rounded-lg hover:bg-gray-800 dark:hover:bg-gray-100 transition-colors"
          >
            {isLast ? 'Finish' : 'Next'}
          </button>
        </div>
      </div>
    </div>
  );
}
