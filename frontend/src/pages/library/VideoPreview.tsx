import { Video as VideoIcon } from 'lucide-react';
import { types } from '../../../wailsjs/go/models';
import { buildLocalFileURL } from './buildLocalFileURL';

interface VideoPreviewProps {
  video: types.Video;
}

export function VideoPreview({ video }: VideoPreviewProps) {
  if (video.status === 'generating') {
    return (
      <div className="mb-5 flex aspect-video items-center justify-center rounded-lg bg-gray-100 text-sm text-gray-500">
        <div className="flex flex-col items-center gap-2">
          <div className="h-2 w-2 animate-ping rounded-full bg-indigo-500" />
          <span>Đang tạo video...</span>
        </div>
      </div>
    );
  }

  const hasFile = video.status === 'completed' && Boolean(video.filePath);
  if (!hasFile) {
    return (
      <div className="mb-5 flex aspect-video flex-col items-center justify-center rounded-lg bg-gray-100 text-center text-sm text-gray-500">
        <VideoIcon className="mb-2 h-8 w-8 text-gray-400" />
        <div className="font-medium text-gray-700">Chưa có file video</div>
        <div className="mt-0.5 max-w-xs text-xs text-gray-500">
          {video.status === 'failed'
            ? 'Tạo video thất bại — không có file để xem.'
            : 'Backend CDP automation chưa hoàn thiện. File sẽ hiển thị ở đây khi tích hợp xong.'}
        </div>
      </div>
    );
  }

  return (
    <div className="mb-5 overflow-hidden rounded-lg bg-black">
      <video
        controls
        preload="metadata"
        src={buildLocalFileURL(video.filePath)}
        className="aspect-video w-full"
      >
        Trình duyệt không hỗ trợ video.
      </video>
    </div>
  );
}
