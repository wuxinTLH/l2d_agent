import java.io.File;
import java.lang.reflect.InvocationTargetException;

/** Opt-in compatibility probe for the user's installed Cubism 5.3.05 reader.
 * Run with a JDK (21+), never bundles or modifies Cubism libraries:
 * java --class-path "Live2DCubism/app/lib/*" scripts/CubismPsdCheck.java FILE.psd
 * Internal class names are version-specific. This is not a supported Editor API
 * or proof that a modeling document was created in the running GUI.
 */
class CubismPsdCheck {
    public static void main(String[] args) {
        try {
            if (args.length != 1) throw new IllegalArgumentException("Expected exactly one PSD path");
            File file = new File(args[0]);
            if (!file.isFile()) throw new IllegalArgumentException("PSD not found");
            Class<?> type = Class.forName("com.live2d.graphics.psd.CPsdDocument");
            Object factory = type.getField("a").get(null);
            Object doc = factory.getClass().getMethod("a", File.class, boolean.class, boolean.class)
                .invoke(factory, file, false, false);
            int width = (Integer) type.getMethod("f").invoke(doc);
            int height = (Integer) type.getMethod("g").invoke(doc);
            Object[] layers = (Object[]) type.getMethod("h").invoke(doc);
            if (width <= 0 || height <= 0 || layers.length == 0)
                throw new IllegalStateException("Reader returned empty document");
            int pixels = 0;
            System.out.println("CANVAS " + width + " " + height);
            for (Object layer : layers) {
                String name = (String) layer.getClass().getMethod("k").invoke(layer);
                java.awt.Rectangle bounds = (java.awt.Rectangle) layer.getClass().getMethod("l").invoke(layer);
                if (bounds.width > 0 && bounds.height > 0) pixels++;
                System.out.println("LAYER " + name + " " + bounds.x + " " + bounds.y + " " + bounds.width + " " + bounds.height);
            }
            if (pixels == 0) throw new IllegalStateException("No nonempty pixel layers");
            System.out.println("CUBISM_READER_PASS pixelLayers=" + pixels);
            // Cubism initializes AWT workers which otherwise keep this probe alive.
            System.exit(0);
        } catch (Throwable error) {
            Throwable cause = error instanceof InvocationTargetException ? error.getCause() : error;
            System.err.println("CUBISM_READER_FAIL " + cause);
            System.exit(1);
        }
    }
}
