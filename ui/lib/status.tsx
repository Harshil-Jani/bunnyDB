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
      return 'bg-red-50 border-red-200 dark:bg-red-900/20 dark:border-red-800';
    case 'WARN':
      return 'bg-yellow-50 border-yellow-200 dark:bg-yellow-900/20 dark:border-yellow-800';
    case 'INFO':
      return 'bg-blue-50 border-blue-200 dark:bg-blue-900/20 dark:border-blue-800';
    default:
      return 'bg-gray-50 border-gray-200 dark:bg-gray-800 dark:border-gray-700';
  }
};

export const getLogLevelIcon = (level: string) => {
  switch (level?.toUpperCase()) {
    case 'ERROR':
      return <XCircle className="w-4 h-4 text-red-500" />;
    case 'WARN':
      return <AlertTriangle className="w-4 h-4 text-yellow-500" />;
    case 'INFO':
      return <Info className="w-4 h-4 text-blue-500" />;
    default:
      return <Info className="w-4 h-4 text-gray-400 dark:text-gray-500" />;
  }
};
