import {useSyncExternalStore} from "react";
import {getRegisteredViews, subscribeRegistry, type ViewPlugin} from "./viewerRegistry";

const EMPTY_VIEWS: readonly ViewPlugin[] = [];

export function useRegisteredViews(): readonly ViewPlugin[] {
    return useSyncExternalStore(
        subscribeRegistry,
        getRegisteredViews,
        () => EMPTY_VIEWS,
    );
}
