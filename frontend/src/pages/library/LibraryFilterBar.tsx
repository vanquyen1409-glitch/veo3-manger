import { Search } from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { Input } from '../../components/ui/Input';
import { Select } from '../../components/ui/Select';
import { StatusFilter } from '../../stores/videosStore';

interface LibraryFilterBarProps {
  search: string;
  onSearch: (q: string) => void;
  statusFilter: StatusFilter;
  onStatusFilter: (f: StatusFilter) => void;
}

export function LibraryFilterBar({
  search,
  onSearch,
  statusFilter,
  onStatusFilter,
}: LibraryFilterBarProps) {
  return (
    <Card>
      <div className="flex flex-wrap items-center gap-3">
        <div className="relative min-w-[240px] flex-1">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" />
          <Input
            placeholder="Tìm theo prompt hoặc tag..."
            value={search}
            onChange={(e) => onSearch(e.target.value)}
            className="pl-9"
          />
        </div>
        <div className="w-48">
          <Select
            value={statusFilter}
            onChange={(e) => onStatusFilter(e.target.value as StatusFilter)}
          >
            <option value="all">Tất cả</option>
            <option value="generating">Đang tạo</option>
            <option value="completed">Hoàn thành</option>
            <option value="failed">Lỗi</option>
          </Select>
        </div>
      </div>
    </Card>
  );
}
