import { type ReactNode } from 'react';
import { motion, AnimatePresence } from 'framer-motion';

interface FloatingBarProps {
  visible: boolean;
  children: ReactNode;
}

export function FloatingBar({ visible, children }: FloatingBarProps) {
  return (
    <AnimatePresence>
      {visible && (
        <motion.div
          initial={{ y: 80, opacity: 0 }}
          animate={{ y: 0, opacity: 1 }}
          exit={{ y: 80, opacity: 0 }}
          transition={{ type: 'spring', damping: 25, stiffness: 300 }}
          className="fixed bottom-6 left-1/2 -translate-x-1/2 z-40"
        >
          <div className="glass rounded-2xl px-6 py-3.5 flex items-center gap-4.5 glow-border">
            {children}
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
