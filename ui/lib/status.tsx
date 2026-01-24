import { CheckCircle, Pause, AlertCircle, RefreshCw, Clock, XCircle, AlertTriangle, Info } from 'lucide-react';

export const getStatusColor = (status: string) => {
  switch (status?.toUpperCase()) {
    case 'RUNNING':
    case 'SYNCED':
      return 'bg-green-100 text-green-800 border-green-200 dark:bg-green-900/30 dark:text-green-400 dark:border-green-800';
    case 'PAUSED':
    case 'PAUSING':
      return 'bg-yellow-100 text-yellow-800 border-yellow-200 dark:bg-yellow-900/30 dark:text-yellow-400 dark:border-yellow-800';
    case 'FAILED':
    case 'ERROR':
      return 'bg-red-100 text-red-800 border-red-200 dark:bg-red-900/30 dark:text-red-400 dark:border-red-800';
    case 'SNAPSHOT':
    case 'RESYNCING':
      return 'bg-blue-100 text-blue-800 border-blue-200 dark:bg-blue-900/30 dark:text-blue-400 dark:border-blue-800';
    case 'SETTING_UP':
    case 'CREATED':
    case 'PENDING':
      return 'bg-gray-100 text-gray-800 border-gray-200 dark:bg-gray-700 dark:text-gray-300 dark:border-gray-600';
    default:
      return 'bg-orange-100 text-orange-800 border-orange-200 dark:bg-orange-900/30 dark:text-orange-400 dark:border-orange-800';
  }
};

export const getStatusIcon = (status: string, size: 'sm' | 'md' = 'sm') => {
  const sizeClass = size === 'sm' ? 'w-4 h-4' : 'w-5 h-5';
  switch (status?.toUpperCase()) {
    case 'RUNNING':
    case 'SYNCED':
      return <CheckCircle className={`${sizeClass} text-green-500`} />;
    case 'PAUSED':
    case 'PAUSING':
      return <Pause className={`${sizeClass} text-yellow-500`} />;
    case 'FAILED':
    case 'ERROR':
      return <AlertCircle className={`${sizeClass} text-red-500`} />;
    case 'SNAPSHOT':
    case 'RESYNCING':
      return <RefreshCw className={`${sizeClass} text-blue-500 animate-spin`} />;
    case 'SETTING_UP':
    case 'CREATED':
    case 'PENDING':
      return <Clock className={`${sizeClass} text-gray-500 dark:text-gray-400`} />;
    default:
      return <RefreshCw className={`${sizeClass} text-orange-500 animate-spin`} />;
  }
};

export const getLogLevelColor = (level: string) => {
  switch (level?.toUpperCase()) {
    case 'ERROR':
      return 'bg-red-50 border-l-red-500 dark:bg-red-950/40 dark:border-l-red-400';
    case 'WARN':
      return 'bg-amber-50 border-l-amber-500 dark:bg-amber-950/40 dark:border-l-amber-400';
    case 'INFO':
      return 'bg-slate-50 border-l-blue-400 dark:bg-slate-800/50 dark:border-l-blue-500';
    case 'DEBUG':
      return 'bg-gray-50 border-l-gray-300 dark:bg-gray-900/50 dark:border-l-gray-600';
    default:
      return 'bg-gray-50 border-l-gray-300 dark:bg-gray-800 dark:border-l-gray-600';
  }
};

export const getLogLevelBadge = (level: string) => {
  switch (level?.toUpperCase()) {
    case 'ERROR':
      return 'bg-red-100 text-red-700 dark:bg-red-900/50 dark:text-red-300';
    case 'WARN':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/50 dark:text-amber-300';
    case 'INFO':
      return 'bg-blue-100 text-blue-700 dark:bg-blue-900/50 dark:text-blue-300';
    case 'DEBUG':
      return 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400';
    default:
      return 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400';
  }
};

export const getLogLevelIcon = (level: string) => {
  switch (level?.toUpperCase()) {
    case 'ERROR':
      return <XCircle className="w-4 h-4 text-red-500 flex-shrink-0" />;
    case 'WARN':
      return <AlertTriangle className="w-4 h-4 text-amber-500 flex-shrink-0" />;
    case 'INFO':
      return <Info className="w-4 h-4 text-blue-500 flex-shrink-0" />;
    case 'DEBUG':
      return <Info className="w-4 h-4 text-gray-400 dark:text-gray-500 flex-shrink-0" />;
    default:
      return <Info className="w-4 h-4 text-gray-400 dark:text-gray-500 flex-shrink-0" />;
  }
};
