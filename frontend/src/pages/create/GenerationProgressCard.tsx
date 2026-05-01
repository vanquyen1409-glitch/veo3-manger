import { Card } from '../../components/ui/Card';
import { STAGE_LABEL } from '../../lib/statuses';
import type { ProgressEvent } from '../../stores/cdpStore';

interface GenerationProgressCardProps {
  progress: ProgressEvent;
}

// Live in-progress indicator shown while a generation is running. Reads the
// stage label from the shared STAGE_LABEL map; falls back to the raw message
// if the stage isn't recognised.
export function GenerationProgressCard({ progress }: GenerationProgressCardProps) {
  return (
    <Card className="border-indigo-200 bg-indigo-50/60">
      <div className="flex items-center gap-3">
        <div className="flex h-2 w-2 rounded-full bg-indigo-500 motion-safe:animate-ping" />
        <div className="flex-1">
          <div className="text-sm font-medium text-indigo-900">
            {STAGE_LABEL[progress.stage] ?? progress.message}
          </div>
          <div className="text-xs text-indigo-700">Video ID: {progress.videoId}</div>
        </div>
      </div>
    </Card>
  );
}
