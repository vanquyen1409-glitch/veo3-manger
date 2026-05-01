import { FormEvent, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { Wand } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { Card, CardHeader } from '../../components/ui/Card';
import { Field } from '../../components/ui/Field';
import { Input } from '../../components/ui/Input';
import { Textarea } from '../../components/ui/Textarea';
import { SegmentedControl } from '../../components/ui/SegmentedControl';
import { TagPills } from '../../components/ui/TagPills';
import { useCDPStore } from '../../stores/cdpStore';
import { useNavStore } from '../../stores/navStore';
import { useVideosStore } from '../../stores/videosStore';
import { types } from '../../../wailsjs/go/models';
import {
  ASPECT_OPTIONS,
  RESOLUTION_OPTIONS,
  DURATION_OPTIONS,
  AspectValue,
  ResolutionValue,
  DurationValue,
} from './options';
import { CDPNotReadyAlert } from './CDPNotReadyAlert';
import { GenerationProgressCard } from './GenerationProgressCard';

export function CreatePage() {
  const [prompt, setPrompt] = useState('');
  const [negativePrompt, setNegativePrompt] = useState('');
  const [aspectRatio, setAspectRatio] = useState<AspectValue>('16:9');
  const [resolution, setResolution] = useState<ResolutionValue>('720p');
  const [duration, setDuration] = useState<DurationValue>(6);
  const [tagsInput, setTagsInput] = useState('');

  const connected = useCDPStore((s) => s.connected);
  const cdpMessage = useCDPStore((s) => s.message);
  const progress = useCDPStore((s) => s.progress);
  const refreshCDP = useCDPStore((s) => s.refresh);

  const setPage = useNavStore((s) => s.setPage);

  const create = useVideosStore((s) => s.create);
  const creating = useVideosStore((s) => s.creating);

  const tags = useMemo(
    () =>
      tagsInput
        .split(',')
        .map((t) => t.trim())
        .filter(Boolean),
    [tagsInput],
  );

  const promptValid = prompt.trim().length > 0;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!connected) {
      toast.error('Chưa kết nối CDP browser', {
        description: 'Vui lòng vào trang Cài đặt và bấm "Kiểm tra kết nối".',
        action: { label: 'Mở Cài Đặt', onClick: () => setPage('settings') },
      });
      return;
    }
    if (!promptValid) {
      toast.error('Prompt không được để trống');
      return;
    }
    try {
      const req = new types.CreateVideoRequest({
        prompt: prompt.trim(),
        negativePrompt: negativePrompt.trim(),
        aspectRatio,
        resolution,
        duration,
        tags,
      });
      const v = await create(req);
      toast.success('Đã thêm video vào hàng đợi', {
        description: `ID: ${v.id}`,
        action: { label: 'Xem Thư Viện', onClick: () => setPage('library') },
      });
      setPrompt('');
      setNegativePrompt('');
      setTagsInput('');
    } catch (err) {
      toast.error('Không tạo được video', { description: String(err) });
    }
  }

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-6">
      <header>
        <h1 className="text-2xl font-semibold text-gray-900">Tạo Video Mới</h1>
        <p className="mt-1 text-sm text-gray-500">
          Nhập prompt và cấu hình, ứng dụng sẽ điều khiển trình duyệt để tạo video tự động.
        </p>
      </header>

      {!connected && (
        <CDPNotReadyAlert
          message={cdpMessage}
          onRefresh={() => void refreshCDP()}
          onOpenSettings={() => setPage('settings')}
        />
      )}

      <form onSubmit={onSubmit} className="flex flex-col gap-6">
        <Card>
          <CardHeader title="Nội dung" description="Mô tả video bạn muốn tạo." />
          <div className="flex flex-col gap-4">
            <Field label="Prompt" required htmlFor="prompt" hint="Mô tả chính cho video.">
              <Textarea
                id="prompt"
                rows={6}
                placeholder="VD: Một con mèo cam đang đi dạo trong khu rừng mưa, ánh sáng vàng dịu nhẹ..."
                value={prompt}
                onChange={(e) => setPrompt(e.target.value)}
                required
              />
            </Field>
            <Field
              label="Negative prompt"
              htmlFor="negative"
              hint="Tùy chọn — những thứ bạn KHÔNG muốn xuất hiện trong video."
            >
              <Textarea
                id="negative"
                rows={3}
                placeholder="VD: mờ, biến dạng, chữ, watermark..."
                value={negativePrompt}
                onChange={(e) => setNegativePrompt(e.target.value)}
              />
            </Field>
          </div>
        </Card>

        <Card>
          <CardHeader title="Cấu hình" description="Tùy chỉnh khung hình, độ phân giải, thời lượng." />
          <div className="grid grid-cols-1 gap-5 md:grid-cols-3">
            <Field label="Tỷ lệ khung hình">
              <SegmentedControl
                value={aspectRatio}
                onChange={setAspectRatio}
                options={ASPECT_OPTIONS.map((o) => ({ value: o.value, label: o.label }))}
              />
            </Field>
            <Field label="Độ phân giải">
              <SegmentedControl
                value={resolution}
                onChange={setResolution}
                options={RESOLUTION_OPTIONS.map((o) => ({ value: o.value, label: o.label }))}
              />
            </Field>
            <Field label="Thời lượng">
              <SegmentedControl
                value={duration}
                onChange={setDuration}
                options={DURATION_OPTIONS.map((o) => ({ value: o.value, label: o.label }))}
              />
            </Field>
          </div>
        </Card>

        <Card>
          <CardHeader title="Tags" description="Tùy chọn — phân cách bằng dấu phẩy để tìm kiếm dễ hơn." />
          <Field label="Tags" htmlFor="tags">
            <Input
              id="tags"
              placeholder="VD: cinematic, slow-motion, nature"
              value={tagsInput}
              onChange={(e) => setTagsInput(e.target.value)}
            />
          </Field>
          {tags.length > 0 && (
            <div className="mt-3">
              <TagPills tags={tags} variant="indigo" />
            </div>
          )}
        </Card>

        {progress && <GenerationProgressCard progress={progress} />}

        <div className="flex items-center justify-end gap-3">
          <Button
            type="submit"
            size="lg"
            loading={creating}
            disabled={!promptValid}
            leftIcon={<Wand className="h-4 w-4" />}
          >
            Tạo Video
          </Button>
        </div>
      </form>
    </div>
  );
}
