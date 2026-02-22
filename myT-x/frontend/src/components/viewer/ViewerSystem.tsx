import {useEffect, useMemo, useState} from "react";
import {ActivityStrip} from "./ActivityStrip";
import {ViewOverlay} from "./ViewOverlay";
import {useViewerStore} from "./viewerStore";
import {getRegisteredViews, subscribeRegistry} from "./viewerRegistry";
import {isImeTransitionalEvent} from "../../utils/ime";

// Side-effect imports: each view self-registers into the registry.
import "./views/file-tree";
import "./views/git-graph";
import "./views/error-log";
import "./views/diff-view";

export function ViewerSystem() {
    const activeViewId = useViewerStore((s) => s.activeViewId);
    const toggleView = useViewerStore((s) => s.toggleView);
    const closeView = useViewerStore((s) => s.closeView);
    const [registryVersion, setRegistryVersion] = useState(0);

    // HMR/runtime-safe registry updates: rebuild memoized view/shortcut state when
    // view modules re-register.
    useEffect(() => {
        return subscribeRegistry(() => {
            setRegistryVersion((version) => version + 1);
        });
    }, []);

    const views = useMemo(() => getRegisteredViews(), [registryVersion]);

    // Shortcut map is derived from the current registry snapshot.
    const shortcutMap = useMemo(() => {
        const map = new Map<string, string>();
        for (const view of views) {
            if (view.shortcut) {
                map.set(view.shortcut.toLowerCase(), view.id);
            }
        }
        return map;
    }, [views]);

    useEffect(() => {
        const handler = (e: KeyboardEvent) => {
            if (e.defaultPrevented) return;
            if (isImeTransitionalEvent(e)) return;

            if (e.key === "Escape" && activeViewId !== null) {
                e.preventDefault();
                closeView();
                return;
            }

            if (e.ctrlKey && e.shiftKey) {
                const combo = `ctrl+shift+${e.key.toLowerCase()}`;
                const viewId = shortcutMap.get(combo);
                if (viewId) {
                    e.preventDefault();
                    toggleView(viewId);
                }
            }
        };

        window.addEventListener("keydown", handler);
        return () => window.removeEventListener("keydown", handler);
    }, [activeViewId, toggleView, closeView, shortcutMap]);

    if (views.length === 0) {
        return null;
    }

    return (
        <>
            <ActivityStrip/>
            <ViewOverlay/>
        </>
    );
}
