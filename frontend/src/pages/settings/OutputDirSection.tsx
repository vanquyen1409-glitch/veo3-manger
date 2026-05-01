import { Folder, FileText } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { Card, CardHeader } from '../../components/ui/Card';
import { Field } from '../../components/ui/Field';
import { Input } from '../../components/ui/Input';

interface OutputDirSectionProps {
  outputDir: string;
  setOutputDir: (v: string) => void;
  onPickFolder: () => void;
  onOpenOutputDir: () => void;
}

export function OutputDirSection({
  outputDir,
  setOutputDir,
  onPickFolder,
  onOpenOutputDir,
}: OutputDirSectionProps) {
  return (
    <Card>
      <CardHeader title="Thư mục lưu video" description="Nơi lưu các file MP4 sau khi tải về." />
      <Field label="Đường dẫn thư mục" htmlFor="outputDir">
        <div className="flex gap-2">
          <Input
            id="outputDir"
            placeholder='C:\Users\...\Videos\Veo3Manager'
            value={outputDir}
            onChange={(e) => setOutputDir(e.target.value)}
          />
          <Button
            type="button"
            variant="secondary"
            onClick={onPickFolder}
            leftIcon={<Folder className="h-4 w-4" />}
          >
            Chọn thư mục
          </Button>
          <Button
            type="button"
            variant="ghost"
            onClick={onOpenOutputDir}
            leftIcon={<FileText className="h-4 w-4" />}
          >
            Mở
          </Button>
        </div>
      </Field>
    </Card>
  );
}
