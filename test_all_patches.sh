for b in nginx-upstream-port-mismatch performance-cliff-hash postgres-migration-schema-drift retry-storm-duplicate-transfer silent-data-corruption split-brain-leader-election timing-attack-comparison wal-fsync-ghost-ack; do
  img="nb-ctrl-$b"
  # Read the last WORKDIR from Dockerfile
  workdir=$(grep '^WORKDIR' benchmarks/$b/Dockerfile | tail -n1 | awk '{print $2}')
  if [ -z "$workdir" ]; then workdir="/workspace"; fi
  echo "Testing $b in $workdir"
  docker run --rm -v $(pwd)/benchmarks/$b:/mnt "$img" sh -c "cd $workdir && git apply /mnt/.bench/solution.patch" || echo "$b FAILED"
done
