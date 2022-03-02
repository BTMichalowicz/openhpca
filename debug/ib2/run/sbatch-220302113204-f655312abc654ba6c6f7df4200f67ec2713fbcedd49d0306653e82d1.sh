#!/bin/bash -l
#
#SBATCH -p thor
#SBATCH -N 4
#SBATCH -t 1:00:00
#SBATCH --error=f655312abc654ba6c6f7df4200f67ec2713fbcedd49d0306653e82d1-220302113204-openmpi4.1.1.err
#SBATCH --output=f655312abc654ba6c6f7df4200f67ec2713fbcedd49d0306653e82d1-220302113204-openmpi4.1.1.out


MPI_DIR=/global/home/users/bwilliams/ompi_x86/build
export PATH=$MPI_DIR/bin:$PATH
export LD_LIBRARY_PATH=$MPI_DIR/lib:$LD_LIBRARY_PATH


which mpirun

mpirun -np 4 --map-by ppr:1:node -rank-by core -bind-to core --mca btl ^openib --mca pml ucx /global/home/users/bwilliams/openhpca/build/install/overlap/overlap/overlap_iallgatherv
