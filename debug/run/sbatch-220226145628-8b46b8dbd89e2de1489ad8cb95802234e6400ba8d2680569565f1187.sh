#!/bin/bash -l
#
#SBATCH -p thor
#SBATCH -N 4
#SBATCH -t 1:00:00
#SBATCH --error=8b46b8dbd89e2de1489ad8cb95802234e6400ba8d2680569565f1187-220226145628-openmpi4.1.1.err
#SBATCH --output=8b46b8dbd89e2de1489ad8cb95802234e6400ba8d2680569565f1187-220226145628-openmpi4.1.1.out


MPI_DIR=/global/home/users/bwilliams/ompi_x86/build
export PATH=$MPI_DIR/bin:$PATH
export LD_LIBRARY_PATH=$MPI_DIR/lib:$LD_LIBRARY_PATH


which mpirun

mpirun -np 4 --map-by ppr:1:node -rank-by core -bind-to core --mca btl ^openib --mca pml ucx -x UCX_NET_DEVICES=mlx5_0:1 /global/home/users/bwilliams/openhpca/build/install/overlap/overlap/overlap_iallgatherv
