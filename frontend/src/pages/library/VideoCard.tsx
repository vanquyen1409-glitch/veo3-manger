import { ReactNode, memo } from 'react';
import { Eye, Trash2, RectangleHorizontal, Clock, Maximize2 } from 'lucide-react';
import { types } from '../../../wailsjs/go/models';
import { Card } from '../../components/ui/Card';
import { StatusBadge } from '../../components/ui/Badge';
import { TagPills } from '../../components/ui/TagPills';
import { formatDate, formatRelative, truncate } from '../../lib/format';

interface VideoCardProps {
  video: types.Video;
  onView: (video: types.Video) => void;
  onDelete: (video: types.Video) => void;
}

function VideoCardImpl({ video, onView, onDelete }: VideoCardProps) {
  return (
    <Card padded={false} className="flex flex-col overflow-hidden transition-shadow hover:shadow-md">
      <div className="flex flex-1 flex-col gap-3 p-4">
        <div className="flex items-start justify-between gap-2">
          <StatusBadge status={video.status} />
          <span className="text-[11px] text-gray-400" title={formatDate(video.createdAt)}>
            {formatRelative(video.createdAt)}
          </span>
        </div>

        <p className="line-clamp-3 text-sm font-medium text-gray-800">
          {truncate(video.prompt, 160)}
        </p>

        <div className="flex flex-wrap gap-2 text-[11px] text-gray-500">
          <Meta icon={<RectangleHorizontal className="h-3 w-3" />} text={video.aspectRatio} />
          <Meta icon={<Maximize2 className="h-3 w-3" />} text={video.resolution} />
          <Meta icon={<Clock className="h-3 w-3" />} text={`${video.duration}s`} />
        </div>

        {video.tags && video.tags.length > 0 && (
          <TagPills tags={video.tags} variant="gray" maxVisible={4} />
        )}

        {video.status === 'failed' && video.errorMessage && (
          <p className="rounded-md bg-rose-50 px-2 py-1.5 text-xs text-rose-700">
            {video.errorMessage}
          </p>
        )}
      </div>

      <div className="flex border-t border-gray-100">
        <button
          type="button"
          onClick={() => onView(video)}
          className="flex flex-1 items-center justify-center gap-1.5 px-3 py-2.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50"
        >
          <Eye className="h-4 w-4" />
          Xem chi tiết
        </button>
        <div className="w-px bg-gray-100" />
        <button
          type="button"
          onClick={() => onDelete(video)}
          className="flex items-center justify-center gap-1.5 px-4 py-2.5 text-sm font-medium text-rose-600 transition-colors hover:bg-rose-50"
        >
          <Trash2 className="h-4 w-4" />
          Xóa
        </button>
      </div>
    </Card>
  );
}

// Memoized so cards in the grid don't re-render when an unrelated card's
// status changes; only re-render when the video object reference changes.
export const VideoCard = memo(VideoCardImpl);

function Meta({ icon, text }: { icon: ReactNode; text: string }) {
  return (
    <span className="inline-flex items-center gap-1 rounded-md bg-gray-50 px-1.5 py-0.5 ring-1 ring-gray-200">
      {icon}
      {text}
    </span>
  );
}

