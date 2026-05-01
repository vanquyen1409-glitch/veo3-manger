import { useCallback, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { Film, Plus, Search } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { ConfirmDialog } from '../components/ui/ConfirmDialog';
import { EmptyState } from '../components/ui/EmptyState';
import { selectFiltered, useVideosStore } from '../stores/videosStore';
import { useNavStore } from '../stores/navStore';
import { truncate } from '../lib/format';
import { types } from '../../wailsjs/go/models';
import { LibraryFilterBar } from './library/LibraryFilterBar';
import { VideoCard } from './library/VideoCard';
import { VideoDetailModal } from './library/VideoDetailModal';

export function LibraryPage() {
  const videos = useVideosStore((s) => s.videos);
  const loading = useVideosStore((s) => s.loading);
  const statusFilter = useVideosStore((s) => s.statusFilter);
  const search = useVideosStore((s) => s.search);
  const setStatusFilter = useVideosStore((s) => s.setStatusFilter);
  const setSearch = useVideosStore((s) => s.setSearch);
  const remove = useVideosStore((s) => s.remove);
  const setPage = useNavStore((s) => s.setPage);

  // Memoize filter locally rather than via `useVideosStore(selectFiltered)`,
  // which would return a fresh array reference on every store change and
  // re-render this page on unrelated state (loading, creating, etc.).
  const filtered = useMemo(
    () => selectFiltered({ videos, statusFilter, search }),
    [videos, statusFilter, search],
  );

  const [confirmTarget, setConfirmTarget] = useState<types.Video | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [detailTarget, setDetailTarget] = useState<types.Video | null>(null);

  const totalLabel = useMemo(() => {
    if (loading) return 'Đang tải...';
    if (videos.length === filtered.length) return `${videos.length} video`;
    return `${filtered.length} / ${videos.length} video`;
  }, [loading, videos.length, filtered.length]);

  // Stable callbacks so memoized VideoCard doesn't re-render the whole grid
  // when an unrelated piece of state changes.
  const onView = useCallback((v: types.Video) => setDetailTarget(v), []);
  const onDelete = useCallback((v: types.Video) => setConfirmTarget(v), []);
  const onCreateFirst = useCallback(() => setPage('create'), [setPage]);

  async function onConfirmDelete() {
    if (!confirmTarget) return;
    setDeleting(true);
    try {
      await remove(confirmTarget.id);
      toast.success('Đã xóa video');
      setConfirmTarget(null);
    } catch (e) {
      toast.error('Không xóa được', { description: String(e) });
    } finally {
      setDeleting(false);
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900">Thư Viện Video</h1>
          <p className="mt-1 text-sm text-gray-500">{totalLabel}</p>
        </div>
        <Button
          variant="primary"
          leftIcon={<Plus className="h-4 w-4" />}
          onClick={() => setPage('create')}
        >
          Tạo Video Mới
        </Button>
      </header>

      <LibraryFilterBar
        search={search}
        onSearch={setSearch}
        statusFilter={statusFilter}
        onStatusFilter={setStatusFilter}
      />

      <LibraryContent
        videos={videos}
        filtered={filtered}
        onView={onView}
        onDelete={onDelete}
        onCreateFirst={onCreateFirst}
      />

      <ConfirmDialog
        open={confirmTarget !== null}
        title="Xóa video?"
        description={
          confirmTarget && (
            <span>
              Hành động này không thể hoàn tác. Video sẽ bị xóa khỏi thư viện.
              <br />
              <span className="mt-2 block text-xs italic text-gray-500">
                "{truncate(confirmTarget.prompt, 80)}"
              </span>
            </span>
          )
        }
        confirmLabel="Xóa"
        destructive
        loading={deleting}
        onConfirm={onConfirmDelete}
        onCancel={() => setConfirmTarget(null)}
      />

      {detailTarget && (
        <VideoDetailModal video={detailTarget} onClose={() => setDetailTarget(null)} />
      )}
    </div>
  );
}

interface LibraryContentProps {
  videos: types.Video[];
  filtered: types.Video[];
  onView: (v: types.Video) => void;
  onDelete: (v: types.Video) => void;
  onCreateFirst: () => void;
}

function LibraryContent({ videos, filtered, onView, onDelete, onCreateFirst }: LibraryContentProps) {
  if (videos.length === 0) {
    return (
      <EmptyState
        icon={<Film className="h-6 w-6" />}
        title="Chưa có video nào"
        description="Bắt đầu bằng cách tạo video đầu tiên của bạn."
        action={
          <Button leftIcon={<Plus className="h-4 w-4" />} onClick={onCreateFirst}>
            Tạo Video Đầu Tiên
          </Button>
        }
      />
    );
  }
  if (filtered.length === 0) {
    return (
      <EmptyState
        icon={<Search className="h-6 w-6" />}
        title="Không có kết quả phù hợp"
        description="Thử đổi bộ lọc hoặc xóa từ khóa tìm kiếm."
      />
    );
  }
  return (
    <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
      {filtered.map((v) => (
        <VideoCard key={v.id} video={v} onView={onView} onDelete={onDelete} />
      ))}
    </div>
  );
}
