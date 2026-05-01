import { Pencil } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { Card, CardHeader } from '../../components/ui/Card';
import { Field } from '../../components/ui/Field';
import { Input } from '../../components/ui/Input';
import { useSettingsStore } from '../../stores/settingsStore';

// Self-contained card for selectors.json — reads path + open action straight
// from the settings store, since this card has no editable fields.
export function SelectorSection() {
  const selectorConfigPath = useSettingsStore((s) => s.settings.selectorConfigPath);
  const openSelectorFile = useSettingsStore((s) => s.openSelectorFile);

  return (
    <Card>
      <CardHeader
        title="Selector Config"
        description="File JSON chứa selector DOM. Mở file để chỉnh sửa khi giao diện Google Flow thay đổi."
      />
      <Field label="Đường dẫn file selector">
        <div className="flex gap-2">
          <Input value={selectorConfigPath} readOnly className="bg-gray-50 font-mono text-xs" />
          <Button
            type="button"
            variant="secondary"
            onClick={() => void openSelectorFile()}
            leftIcon={<Pencil className="h-4 w-4" />}
          >
            Mở file
          </Button>
        </div>
      </Field>
    </Card>
  );
}
