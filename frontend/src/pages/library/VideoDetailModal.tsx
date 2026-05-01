import { ReactNode, useEffect } from 'react';
import { FolderOpen, X } from 'lucide-react';
import { OpenPathInOS } from '../../../wailsjs/go/main/App';
import { types } from '../../../wailsjs/go/models';
import { StatusBadge } from '../../components/ui/Badge';
import { TagPills } from '../../components/ui/TagPills';
import { formatDate } from '../../lib/format';
import { VideoPreview } from './VideoPreview';

interface VideoDetailModalProps {
  video: types.Video;
  onClose: () => void;
}

export function VideoDetailModal({ video, onClose }: VideoDetailModalProps) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [onClose]);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 bg-gray-900/50" onClick={onClose} />
      <div className="relative w-full max-w-2xl overflow-hidden rounded-xl bg-white shadow-xl">
        <ModalHeader video={video} onClose={onClose} />
        <div className="max-h-[70vh] overflow-y-auto p-6">
          <VideoPreview video={video} />
          <ModalBody video={video} />
        </div>
      </div>
    </div>
  );
}

function ModalHeader({ video, onClose }: { video: types.Video; onClose: () => void }) {
  return (
    <div className="flex items-start justify-between gap-3 border-b border-gray-100 px-6 py-4">
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <h3 className="text-base font-semibold text-gray-900">Chi tiết video</h3>
          <StatusBadge status={video.status} />
        </div>
        <p className="mt-0.5 text-xs text-gray-500">{video.id}</p>
      </div>
      <button
        type="button"
        onClick={onClose}
        className="rounded-md p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700"
      >
        <X className="h-4 w-4" />
      </button>
    </div>
  );
}

function ModalBody({ video }: { video: types.Video }) {
  return (
    <>
      <DetailRow label="Prompt" value={video.prompt} multiline />
      {video.negativePrompt && (
        <DetailRow label="Negative prompt" value={video.negativePrompt} multiline />
      )}
      <div className="grid grid-cols-3 gap-4">
        <DetailRow label="Tỷ lệ" value={video.aspectRatio} />
        <DetailRow label="Độ phân giải" value={video.resolution} />
        <DetailRow label="Thời lượng" value={`${video.duration}s`} />
      </div>
      {video.tags && video.tags.length > 0 && (
        <DetailRow label="Tags" value={<TagPills tags={video.tags} variant="indigo" />} />
      )}
      <DetailRow label="Tạo lúc" value={formatDate(video.createdAt)} />
      {video.completedAt && <DetailRow label="Hoàn tất lúc" value={formatDate(video.completedAt)} />}
      {video.filePath && <FilePathRow filePath={video.filePath} />}
      {video.errorMessage && (
        <DetailRow label="Lỗi" value={<span className="text-rose-700">{video.errorMessage}</span>} />
      )}
    </>
  );
}

function FilePathRow({ filePath }: { filePath: string }) {
  return (
    <DetailRow
      label="File"
      value={
        <div className="flex items-center justify-between gap-2">
          <span className="break-all font-mono text-xs">{filePath}</span>
          <button
            type="button"
            onClick={() => void OpenPathInOS(filePath)}
            className="inline-flex shrink-0 items-center gap-1 rounded-md border border-gray-300 bg-white px-2 py-1 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-50"
          >
            <FolderOpen className="h-3.5 w-3.5" />
            Mở
          </button>
        </div>
      }
    />
  );
}

interface DetailRowProps {
  label: string;
  value: ReactNode;
  multiline?: boolean;
}

function DetailRow({ label, value, multiline = false }: DetailRowProps) {
  return (
    <div className="mb-4 last:mb-0">
      <div className="text-xs font-medium uppercase tracking-wide text-gray-500">{label}</div>
      <div className={['mt-1 text-sm text-gray-900', multiline ? 'whitespace-pre-wrap' : ''].join(' ')}>
        {value}
      </div>
    </div>
  );
}
